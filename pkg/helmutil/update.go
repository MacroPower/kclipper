package helmutil

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

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kclipper/pkg/kclchart"
	"github.com/MacroPower/kclipper/pkg/log"
)

var (
	ErrUpdateWorkerFailed = errors.New("update worker failed")
	ErrChartUpdateFailed  = errors.New("chart update failed")
	ErrKCLExecutionFailed = errors.New("kcl execution failed")
)

func (c *ChartPkg) UpdateTUI(logLevel string, charts ...string) error {
	c.p = tea.NewProgram(newUpdateTUI())

	logger, err := log.CreateHandler(c, logLevel, log.FormatText)
	if err != nil {
		return fmt.Errorf("failed to create log handler: %w", err)
	}

	slog.SetDefault(slog.New(logger))

	go func() {
		err := c.Update(charts...)
		if err != nil {
			c.SendUpdate(fmt.Errorf("%w: %w", ErrChartUpdateFailed, err))
		}
	}()

	if _, err := c.p.Run(); err != nil {
		return fmt.Errorf("failed to launch tui: %w", err)
	}

	return nil
}

// Update loads the chart configurations defined in charts.k and calls Add to
// generate all required chart packages.
func (c *ChartPkg) Update(charts ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

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
		return fmt.Errorf("%w: %w", ErrKCLExecutionFailed, err)
	}

	errMsg := mainOutput.GetErrMessage()
	if errMsg != "" {
		return fmt.Errorf("%w: %s", ErrKCLExecutionFailed, errMsg)
	}

	mainData := mainOutput.GetJsonResult()
	chartData := &kclchart.ChartData{}

	if err := json.Unmarshal([]byte(mainData), chartData); err != nil {
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

	c.SendUpdate(teaMsgSetChartTotal(chartCount))

	for _, k := range chartData.GetSortedKeys() {
		chart := chartData.Charts[k]
		chartName := chart.Chart

		chartSlog := slog.With(
			slog.String("chart_name", chartName),
			slog.String("chart_key", k),
		)

		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("%w: %w", ErrUpdateWorkerFailed, err)
		}
		c.SendUpdate(teaMsgUpdatingChart(k))
		go func(chart kclchart.ChartConfig, logger *slog.Logger) {
			defer sem.Release(1)

			logger.Info("updating chart")

			err := c.AddChart(k, &chart)
			if err != nil {
				c.SendUpdate(teaMsgUpdatedChart{chart: k, err: err})
				errChan <- fmt.Errorf("update %q: %w", k, err)

				return
			}

			c.SendUpdate(teaMsgUpdatedChart{chart: k})

			logger.Info("finished updating chart")
		}(chart, chartSlog)
	}

	if err := sem.Acquire(ctx, workerCount); err != nil {
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

	slog.Info("update complete")

	return nil
}
