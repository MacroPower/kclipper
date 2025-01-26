package helmutil

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"kcl-lang.io/cli/pkg/options"
	"kcl-lang.io/kcl-go/pkg/kcl"

	"github.com/MacroPower/kclipper/pkg/kclchart"
)

// Update loads the chart configurations defined in charts.k and calls Add to
// generate all required chart packages.
func (c *ChartPkg) Update(charts ...string) error {
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
		chartName := chart.Chart
		chartKey := chart.GetSnakeCaseName()
		if k != chartKey {
			return fmt.Errorf("chart key '%s' does not match generated key '%s'", k, chartKey)
		}
		if len(charts) > 0 && !slices.Contains(charts, chartName) && !slices.Contains(charts, chartKey) {
			slog.Info("skipping chart", slog.String("name", chartName), slog.String("key", chartKey))
			continue
		}
		slog.Info("updating chart", slog.String("name", chartName), slog.String("key", chartKey))
		err := c.AddChart(&chart)
		if err != nil {
			return fmt.Errorf("failed to update chart '%s': %w", k, err)
		}
	}

	return nil
}
