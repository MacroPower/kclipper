package helmutil

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
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

	slog.Debug("running kcl",
		slog.String("path", c.BasePath),
		slog.String("deps", fmt.Sprint(externalPkgs)),
	)

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

	workerCount := int64(runtime.NumCPU())
	chartCount := int64(len(chartData.Charts))
	if workerCount > chartCount {
		workerCount = chartCount
	}
	sem := semaphore.NewWeighted(workerCount)
	errChan := make(chan error, len(chartData.Charts))

	for _, k := range chartData.GetSortedKeys() {
		chart := chartData.Charts[k]
		chartName := chart.Chart

		chartSlog := slog.With(
			slog.String("chart_name", chartName),
			slog.String("chart_key", k),
		)

		err = sem.Acquire(context.Background(), 1)
		if err != nil {
			return fmt.Errorf("failed to acquire worker: %w", err)
		}
		go func(chart kclchart.ChartConfig, logger *slog.Logger) {
			defer sem.Release(1)

			if len(charts) > 0 && !slices.Contains(charts, chartName) && !slices.Contains(charts, k) {
				chartSlog.Info("skipping chart")

				return
			}

			logger.Info("updating chart")

			err := c.AddChart(k, &chart)
			if err != nil {
				errChan <- fmt.Errorf("failed to update chart %q: %w", k, err)

				return
			}
		}(chart, chartSlog)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	err = sem.Acquire(ctx, workerCount)
	if err != nil {
		return fmt.Errorf("failed to update charts: %w", err)
	}

	close(errChan)
	var merr error
	for err := range errChan {
		merr = multierror.Append(merr, err)
	}
	if merr != nil {
		return fmt.Errorf("failed to update charts: %w", err)
	}

	slog.Info("update complete")

	return nil
}
