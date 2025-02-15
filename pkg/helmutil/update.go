package helmutil

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"

	"kcl-lang.io/kcl-go/pkg/native"
	"kcl-lang.io/kcl-go/pkg/spec/gpyrpc"

	"github.com/MacroPower/kclipper/pkg/kclchart"
)

// Update loads the chart configurations defined in charts.k and calls Add to
// generate all required chart packages.
func (c *ChartPkg) Update(charts ...string) error {
	svc := native.NewNativeServiceClient()

	absBasePath, err := filepath.Abs(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	slog.Debug("updating kcl dependencies")

	depOutput, err := svc.UpdateDependencies(&gpyrpc.UpdateDependencies_Args{
		ManifestPath: absBasePath,
		Vendor:       c.Vendor,
	})
	if err != nil {
		return fmt.Errorf("failed to update dependencies: %w", err)
	}

	externalPkgs := depOutput.GetExternalPkgs()

	slog.Debug("running kcl", slog.String("path", c.BasePath), slog.String("deps", fmt.Sprint(externalPkgs)))

	mainOutput, err := svc.ExecProgram(&gpyrpc.ExecProgram_Args{
		WorkDir:       absBasePath,
		KFilenameList: []string{"."},
		FastEval:      c.FastEval,
		ExternalPkgs:  externalPkgs,
	})
	if err != nil {
		return fmt.Errorf("failed to execute kcl: %w", err)
	}

	errMsg := mainOutput.GetErrMessage()
	if errMsg != "" {
		return fmt.Errorf("failed to execute kcl: %s", errMsg)
	}

	mainData := mainOutput.GetJsonResult()
	chartData := &kclchart.ChartData{}

	if err := json.Unmarshal([]byte(mainData), chartData); err != nil {
		return fmt.Errorf("failed to unmarshal output: %w", err)
	}

	updatedCount := 0

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

		updatedCount++
	}

	slog.Info("update complete", slog.Int("updated", updatedCount))

	return nil
}
