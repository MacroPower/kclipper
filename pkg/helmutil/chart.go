package helmutil

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/kclutil"
)

type ChartPkg struct {
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

func NewChartPkg(basePath string, client helm.ChartClient, opts ...ChartPkgOpts) (*ChartPkg, error) {
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get absolute path: %w", ErrPathResolution, err)
	}

	slog.Debug("looking for repository root", slog.String("path", basePath))
	repoRoot, err := kclutil.FindRepoRoot(basePath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to find repository root: %w", ErrPathResolution, err)
	}
	slog.Debug("found repository root", slog.String("path", repoRoot))

	c := &ChartPkg{
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
	pkgPath, err := kclutil.FindTopPkgRoot(repoRoot, basePath)
	if errors.Is(err, kclutil.ErrFileNotFound) {
		slog.Warn("kcl.mod file not found, creating a new one")
		_, err = c.Init()
		if err != nil {
			return nil, fmt.Errorf("call chart init: %w", err)
		}
		pkgPath, err = kclutil.FindTopPkgRoot(repoRoot, basePath)
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

type ChartPkgOpts func(*ChartPkg)

func WithVendor(vendor bool) ChartPkgOpts {
	return func(c *ChartPkg) {
		c.Vendor = vendor
	}
}

func WithFastEval(fastEval bool) ChartPkgOpts {
	return func(c *ChartPkg) {
		c.FastEval = fastEval
	}
}

func WithTimeout(timeout time.Duration) ChartPkgOpts {
	return func(c *ChartPkg) {
		c.Timeout = timeout
	}
}

func WithMaxExtractSize(size *resource.Quantity) ChartPkgOpts {
	return func(c *ChartPkg) {
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

func (c *ChartPkg) broadcastEvent(evt any) {
	for _, sub := range c.subs {
		sub(evt)
	}
}

func (c *ChartPkg) Subscribe(f func(any)) {
	c.subs = append(c.subs, f)
}

func (c *ChartPkg) updateFile(automation kclutil.Automation, kclFile, initialContents, specPath string) error {
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

	_, err = kcl.OverrideFile(kclFile, specs, imports)
	if err != nil {
		return fmt.Errorf("failed to update %q: %w", kclFile, err)
	}

	return nil
}
