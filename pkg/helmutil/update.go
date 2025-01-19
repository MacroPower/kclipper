package helmutil

import (
	"encoding/json"
	"fmt"

	"kcl-lang.io/cli/pkg/options"
	"kcl-lang.io/kcl-go/pkg/kcl"

	"github.com/MacroPower/kclipper/pkg/kclchart"
)

// Update loads the chart configurations defined in charts.k and calls Add to
// generate all required chart packages.
func (c *ChartPkg) Update() error {
	depOpt, err := options.LoadDepsFrom(c.BasePath, true)
	if err != nil {
		return fmt.Errorf("failed to load KCL dependencies: %w", err)
	}

	mainOutput, err := kcl.Run(c.BasePath, *depOpt)
	if err != nil {
		return fmt.Errorf("failed to run '%s': %w", c.BasePath, err)
	}

	mainData := mainOutput.GetRawJsonResult()

	chartData := &kclchart.ChartData{}
	if err := json.Unmarshal([]byte(mainData), chartData); err != nil {
		return fmt.Errorf("failed to unmarshal output from '%s': %w", c.BasePath, err)
	}

	for _, k := range chartData.GetSortedKeys() {
		chart := chartData.Charts[k]
		if k != chart.GetSnakeCaseName() {
			return fmt.Errorf("chart key '%s' does not match chart name '%s'", k, chart.GetSnakeCaseName())
		}
		err := c.Add(chart.Chart, chart.RepoURL, chart.TargetRevision, chart.SchemaPath,
			chart.CRDPath, chart.SchemaGenerator, chart.SchemaValidator, chart.Repositories)
		if err != nil {
			return fmt.Errorf("failed to update chart '%s': %w", k, err)
		}
	}

	return nil
}
