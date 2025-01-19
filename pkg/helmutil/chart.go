package helmutil

import (
	"fmt"
	"os"
	"sync"

	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/kclutil"
)

type ChartPkg struct {
	BasePath string
	Client   helm.ChartFileClient

	mu sync.RWMutex
}

func NewChartPkg(basePath string, client helm.ChartFileClient) *ChartPkg {
	return &ChartPkg{
		BasePath: basePath,
		Client:   client,
	}
}

func fileExists(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	return true
}

func (c *ChartPkg) updateFile(automation kclutil.Automation, kclFile, initialContents, specPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !fileExists(kclFile) {
		if err := os.WriteFile(kclFile, []byte(initialContents), 0o600); err != nil {
			return fmt.Errorf("failed to write '%s': %w", kclFile, err)
		}
	}

	specs, err := automation.GetSpecs(specPath)
	if err != nil {
		return fmt.Errorf("failed generating inputs for '%s': %w", kclFile, err)
	}

	imports := []string{"helm"}
	_, err = kcl.OverrideFile(kclFile, specs, imports)
	if err != nil {
		return fmt.Errorf("failed to update '%s': %w", kclFile, err)
	}

	return nil
}
