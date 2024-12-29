package helmutil

import (
	"fmt"
	"path"

	"kcl-lang.io/cli/pkg/options"
	"kcl-lang.io/kcl-go/pkg/kcl"

	helmchart "github.com/MacroPower/kclx/pkg/helm/models"
)

type ChartData struct {
	Charts map[string]helmchart.ChartConfig `json:"charts"`
}

// Update loads the chart configurations defined in main.k and calls Add to
// generate all required chart packages.
func (c *ChartPkg) Update() error {
	depOpt, err := options.LoadDepsFrom(c.BasePath, true)
	if err != nil {
		return fmt.Errorf("failed to load KCL dependencies: %w", err)
	}

	mainFile := path.Join(c.BasePath, "main.k")
	options := []kcl.Option{
		*depOpt,
	}
	mainOutput, err := kcl.Run(mainFile, options...)
	if err != nil {
		return fmt.Errorf("failed to run '%s': %w", mainFile, err)
	}

	chartData := &ChartData{}

	err = mainOutput.ToType(chartData)
	if err != nil {
		return fmt.Errorf("failed to convert main.k output to struct: %w", err)
	}

	return nil
}
