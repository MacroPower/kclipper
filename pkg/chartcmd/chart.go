package chartcmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/kclautomation"
	"github.com/MacroPower/kclipper/pkg/kclerrors"
	"github.com/MacroPower/kclipper/pkg/paths"
)

type KCLPackage struct {
	Client         helm.ChartClient
	MaxExtractSize *resource.Quantity
	BasePath       string
	absBasePath    string
	pkgPath        string
	repoRoot       string
	subs           []func(any)
	Timeout        time.Duration
	mu             sync.RWMutex
	Vendor         bool
	FastEval       bool
}

func NewKCLPackage(basePath string, client helm.ChartClient, opts ...KCLPackageOpts) (*KCLPackage, error) {
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get absolute path: %w", ErrPathResolution, err)
	}

	slog.Debug("looking for repository root", slog.String("path", basePath))
	repoRoot, err := paths.FindRepoRoot(basePath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to find repository root: %w", ErrPathResolution, err)
	}
	slog.Debug("found repository root", slog.String("path", repoRoot))

	c := &KCLPackage{
		Vendor:         false,
		FastEval:       true,
		BasePath:       basePath,
		absBasePath:    absBasePath,
		repoRoot:       repoRoot,
		Client:         client,
		MaxExtractSize: resource.NewQuantity(10485760, resource.BinarySI), // 10Mi.
		Timeout:        5 * time.Minute,
		subs:           []func(any){},
	}
	for _, opt := range opts {
		opt(c)
	}

	slog.Debug("looking for topmost kcl.mod file",
		slog.String("begin", absBasePath),
		slog.String("end", repoRoot),
	)
	pkgPath, err := paths.FindTopPkgRoot(repoRoot, basePath)
	if errors.Is(err, kclerrors.ErrFileNotFound) {
		slog.Warn("kcl.mod file not found, creating a new one")
		_, err = c.Init()
		if err != nil {
			return nil, fmt.Errorf("call chart init: %w", err)
		}
		pkgPath, err = paths.FindTopPkgRoot(repoRoot, basePath)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to find package root; could not recover after init: %w", ErrPathResolution, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("%w: failed to find package root: %w", ErrPathResolution, err)
	}
	slog.Debug("found topmost kcl.mod file", slog.String("path", pkgPath))
	c.pkgPath = pkgPath

	return c, nil
}

type KCLPackageOpts func(*KCLPackage)

func WithVendor(vendor bool) KCLPackageOpts {
	return func(c *KCLPackage) {
		c.Vendor = vendor
	}
}

func WithFastEval(fastEval bool) KCLPackageOpts {
	return func(c *KCLPackage) {
		c.FastEval = fastEval
	}
}

func WithTimeout(timeout time.Duration) KCLPackageOpts {
	return func(c *KCLPackage) {
		c.Timeout = timeout
	}
}

func WithMaxExtractSize(size *resource.Quantity) KCLPackageOpts {
	return func(c *KCLPackage) {
		c.MaxExtractSize = size
	}
}

func fileExists(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil || fi.IsDir() {
		return false
	}

	return true
}

func (c *KCLPackage) broadcastEvent(evt any) {
	for _, sub := range c.subs {
		sub(evt)
	}
}

func (c *KCLPackage) Subscribe(f func(any)) {
	c.subs = append(c.subs, f)
}

func (c *KCLPackage) updateFile(automation kclautomation.Automation, kclFile, initialContents, specPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !fileExists(kclFile) {
		if err := os.WriteFile(kclFile, []byte(initialContents), 0o600); err != nil {
			return fmt.Errorf("failed to write %q: %w", kclFile, err)
		}
	}

	specs, err := automation.GetSpecs(specPath)
	if err != nil {
		return fmt.Errorf("failed generating inputs for %q: %w", kclFile, err)
	}

	imports := []string{"helm"}

	_, err = kclautomation.File.OverrideFile(kclFile, specs, imports)
	if err != nil {
		return fmt.Errorf("failed to update %q: %w", kclFile, err)
	}

	return nil
}
