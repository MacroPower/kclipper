package helmutil

import (
	"encoding/json"
	"fmt"
	"path"

	"kcl-lang.io/cli/pkg/options"
	"kcl-lang.io/kcl-go/pkg/kcl"

	"github.com/MacroPower/kclx/pkg/helmmodels"
)

// Update loads the chart configurations defined in charts.k and calls Add to
// generate all required chart packages.
func (c *ChartPkg) Update() error {
	depOpt, err := options.LoadDepsFrom(c.BasePath, true)
	if err != nil {
		return fmt.Errorf("failed to load KCL dependencies: %w", err)
	}

	mainFile := path.Join(c.BasePath, "charts.k")
	mainOutput, err := kcl.Run(mainFile, *depOpt)
	if err != nil {
		return fmt.Errorf("failed to run '%s': %w", mainFile, err)
	}

	mainData := mainOutput.GetRawJsonResult()

	chartData := &helmmodels.ChartData{}
	if err := json.Unmarshal([]byte(mainData), chartData); err != nil {
		return fmt.Errorf("failed to unmarshal output from '%s': %w", mainFile, err)
	}

	for k, chart := range chartData.Charts {
		if k != chart.GetSnakeCaseName() {
			return fmt.Errorf("chart key '%s' does not match chart name '%s'", k, chart.GetSnakeCaseName())
		}
		err := c.Add(chart.Chart, chart.RepoURL, chart.TargetRevision,
			chart.SchemaPath, chart.SchemaGenerator, chart.SchemaValidator)
		if err != nil {
			return fmt.Errorf("failed to update chart '%s': %w", k, err)
		}
	}

	return nil
}
