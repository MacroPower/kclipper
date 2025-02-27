package helmutil

import (
	"fmt"
	"os"
	"sync"
	"time"

	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/kclutil"
)

type ChartPkg struct {
	Client   helm.ChartFileClient
	BasePath string
	subs     []func(any)
	Timeout  time.Duration
	mu       sync.RWMutex
	Vendor   bool
	FastEval bool
}

func NewChartPkg(basePath string, client helm.ChartFileClient, opts ...ChartPkgOpts) *ChartPkg {
	c := &ChartPkg{
		Vendor:   false,
		FastEval: true,
		BasePath: basePath,
		Client:   client,
		Timeout:  5 * time.Minute,
		subs:     []func(any){},
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
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
