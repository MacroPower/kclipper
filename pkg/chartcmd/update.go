package chartcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"runtime"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
	"kcl-lang.io/kcl-go/pkg/native"
	"kcl-lang.io/kcl-go/pkg/spec/gpyrpc"

	"github.com/MacroPower/kclipper/pkg/kclmodule/kclchart"
)

var (
	ErrUpdateWorkerFailed = errors.New("update worker failed")
	ErrChartUpdateFailed  = errors.New("chart update failed")
	ErrKCLExecutionFailed = errors.New("kcl execution failed")
)

// Update loads the chart configurations defined in charts.k and calls Add to
// generate all required chart packages.
func (c *KCLPackage) Update(charts ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	logger := slog.With(
		slog.String("cmd", "chart_update"),
	)

	svc := native.NewNativeServiceClient()

	absBasePath, err := filepath.Abs(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %q: %w", c.BasePath, err)
	}

	logger.Debug("updating kcl dependencies",
		slog.String("path", absBasePath),
		slog.Bool("vendor", c.Vendor),
	)

	depOutput, err := svc.UpdateDependencies(&gpyrpc.UpdateDependencies_Args{
		ManifestPath: absBasePath,
		Vendor:       c.Vendor,
	})
	if err != nil {
		return fmt.Errorf("failed to update dependencies at %q: %w", absBasePath, err)
	}

	externalPkgs := depOutput.GetExternalPkgs()

	logger.Debug("running kcl",
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
		return fmt.Errorf("%w: %w", ErrKCLExecutionFailed, err)
	}

	errMsg := mainOutput.GetErrMessage()
	if errMsg != "" {
		return fmt.Errorf("%w: %s", ErrKCLExecutionFailed, errMsg)
	}

	mainData := mainOutput.GetJsonResult()
	chartData := &kclchart.ChartData{}

	err = json.Unmarshal([]byte(mainData), chartData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal output: %w", err)
	}

	if len(charts) > 0 {
		matchedCharts := map[string]kclchart.ChartConfig{}
		for _, chart := range charts {
			vk, ok := chartData.GetByKey(chart)
			vn := chartData.FilterByName(chart)
			if !ok && len(vn) == 0 {
				return fmt.Errorf("chart %q not found", chart)
			}

			maps.Copy(matchedCharts, vn)
			if ok {
				matchedCharts[chart] = vk
			}
		}

		chartData.Charts = matchedCharts
	}

	workerCount := int64(runtime.GOMAXPROCS(0))
	chartCount := len(chartData.Charts)
	sem := semaphore.NewWeighted(workerCount)
	errChan := make(chan error, chartCount)

	c.broadcastEvent(EventSetChartTotal(chartCount))

	for _, k := range chartData.GetSortedKeys() {
		chart := chartData.Charts[k]
		chartName := chart.Chart

		chartLogger := logger.With(
			slog.String("chart_name", chartName),
			slog.String("chart_key", k),
		)

		err := sem.Acquire(ctx, 1)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrUpdateWorkerFailed, err)
		}

		c.broadcastEvent(EventUpdatingChart(k))

		go func() {
			defer sem.Release(1)

			chartLogger.Info("updating chart")

			err := c.AddChart(k, &chart)
			if err != nil {
				c.broadcastEvent(EventUpdatedChart{Chart: k, Err: err})

				errChan <- fmt.Errorf("update %q: %w", k, err)

				return
			}

			c.broadcastEvent(EventUpdatedChart{Chart: k})

			chartLogger.Info("finished updating chart")
		}()
	}

	err = sem.Acquire(ctx, workerCount)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUpdateWorkerFailed, err)
	}

	close(errChan)

	var merr error
	for err := range errChan {
		merr = multierror.Append(merr, err)
	}
	if merr != nil {
		return merr
	}

	logger.Info("update complete")

	return nil
}
