package helmutil

import (
	"fmt"
	"os"
	"path"
	"sync"

	"kcl-lang.io/kcl-go"
	kclutil "kcl-lang.io/kcl-go/pkg/utils"
)

type ChartPkg struct {
	BasePath string

	mu sync.RWMutex
}

func NewChartPkg(basePath string) *ChartPkg {
	return &ChartPkg{
		BasePath: basePath,
	}
}

func (c *ChartPkg) updateMainFile(vendorDir, chartKey string, chartConfig ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	mainFile := path.Join(vendorDir, "main.k")
	if !kclutil.FileExists(mainFile) {
		if err := os.WriteFile(mainFile, []byte(initialMainContents), 0o600); err != nil {
			return fmt.Errorf("failed to write '%s': %w", mainFile, err)
		}
	}
	imports := []string{"helm"}
	specs := []string{}
	for _, cc := range chartConfig {
		specs = append(specs, fmt.Sprintf(`charts.%s.%s`, chartKey, cc))
	}
	_, err := kcl.OverrideFile(mainFile, specs, imports)
	if err != nil {
		return fmt.Errorf("failed to update '%s': %w", mainFile, err)
	}
	return nil
}
