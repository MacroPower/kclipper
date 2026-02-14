package helm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/macropower/kclipper/pkg/helmrepo"
)

var (
	ErrChartDependency   = errors.New("error in chart dependency")
	ErrChartWorkerFailed = errors.New("chart worker failed")
)

// PulledChart represents a Helm chart.tar.gz, or the root directory of a Helm
// chart. It is typically created via [Client.Pull].
type PulledChart struct {
	repos  helmrepo.Getter
	client ChartClient
	chart  string
	path   string
}

// Extract will extract the chart (if it is a .tar.gz file), and return the path
// to the extracted chart. An [io.Closer] is also returned, calling Close() will
// clean up the extracted chart. If [PulledChart] references a directory, the
// the path to the directory and a [NopCloser] is returned.
func (c *PulledChart) Extract(maxSize *resource.Quantity) (string, io.Closer, error) {
	closer := NewNopCloser()

	// If the chart is already extracted, return the path to the extracted chart.
	if dirExists(c.path) {
		return c.path, closer, nil
	}

	extractedPath, closer, err := c.extractChart(c.chart, c.path, maxSize)
	if err != nil {
		return "", nil, fmt.Errorf("extract chart: %w", err)
	}

	return extractedPath, closer, nil
}

// Load will load the Helm chart into a [chart.Chart]. If [PulledChart]
// references a .tar.gz, it will be loaded directly into memory without
// extracting the files to disk. If [PulledChart] references a directory, the
// contents of the chart will be loaded from the filesystem. No closer is
// returned by this method, since no temporary files are created.
func (c *PulledChart) Load(ctx context.Context, skipSchemaValidation bool) (*chart.Chart, error) {
	loadedChart, err := loader.Load(c.path)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	// Keeping the schema in the charts will cause downstream templating to load
	// remote refs and validate against the schema, for the chart and all its
	// dependencies. This can be a massive and random-feeling performance hit,
	// and is largely unnecessary since KCL will be using the same, or a similar
	// schema to validate the values.
	if skipSchemaValidation {
		removeSchemasFromObject(loadedChart)
	}

	// Recursively load and set all chart dependencies.
	err = c.loadChartDependencies(ctx, loadedChart, skipSchemaValidation)
	if err != nil {
		return nil, fmt.Errorf("load chart dependencies: %w", err)
	}

	return loadedChart, nil
}

func (c *PulledChart) extractChart(chartName, srcPath string, maxSize *resource.Quantity) (string, io.Closer, error) {
	tempDest, err := createTempDir(os.TempDir())
	if err != nil {
		return "", nil, fmt.Errorf("create temporary directory: %w", err)
	}

	//nolint:gosec // G304 checked by repo resolver.
	reader, err := os.Open(srcPath)
	if err != nil {
		return "", nil, fmt.Errorf("open chart path %q: %w", srcPath, err)
	}

	err = gunzip(tempDest, reader, maxSize.Value(), false)
	if err != nil {
		_ = os.RemoveAll(tempDest)

		return "", nil, fmt.Errorf("gunzip chart: %w", err)
	}

	return filepath.Join(tempDest, normalizeChartName(chartName)), newInlineCloser(func() error {
		return os.RemoveAll(tempDest)
	}), nil
}

// loadChartDependencies concurrently loads and sets the dependencies of the
// target chart. It is called recursively until all dependencies are loaded.
// It uses a weighted semaphore to limit the number of concurrent loads.
func (c *PulledChart) loadChartDependencies(ctx context.Context, target *chart.Chart, skipSchemaValidation bool) error {
	workerCount := int64(runtime.GOMAXPROCS(0))
	sem := semaphore.NewWeighted(workerCount)

	return c.setChartDependencies(ctx, target, sem, skipSchemaValidation)
}

func (c *PulledChart) setChartDependencies(
	ctx context.Context,
	target *chart.Chart,
	sem *semaphore.Weighted,
	skipSchemaValidation bool,
) error {
	loadedDeps := []*chart.Chart{}

	type loadResult struct {
		chart *chart.Chart
		err   error
	}

	depCount := int64(len(target.Metadata.Dependencies))
	resultCh := make(chan loadResult, depCount)
	// The smaller semaphore of sem and innerSem will block.
	innerSem := semaphore.NewWeighted(depCount)

	for _, chartDep := range target.Metadata.Dependencies {
		err := sem.Acquire(ctx, 1)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrChartWorkerFailed, err)
		}

		err = innerSem.Acquire(ctx, 1)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrChartWorkerFailed, err)
		}

		go func() {
			defer sem.Release(1)
			defer innerSem.Release(1)

			dep, err := c.getChartDependency(ctx, target, chartDep)
			if err != nil {
				resultCh <- loadResult{err: fmt.Errorf("get dependency %q: %w", target.Name(), err)}

				return
			}

			if skipSchemaValidation {
				removeSchemasFromObject(dep)
			}

			resultCh <- loadResult{chart: dep}
		}()
	}

	err := innerSem.Acquire(ctx, depCount)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrChartWorkerFailed, err)
	}

	close(resultCh)

	var merr error
	for result := range resultCh {
		if result.err != nil {
			merr = multierror.Append(merr, result.err)

			continue
		}

		err := c.setChartDependencies(ctx, result.chart, sem, skipSchemaValidation)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrChartDependency, err)
		}

		loadedDeps = append(loadedDeps, result.chart)
	}

	if merr != nil {
		return merr
	}

	target.SetDependencies(loadedDeps...)

	return nil
}

func (c *PulledChart) getChartDependency(
	ctx context.Context,
	parentChart *chart.Chart,
	dep *chart.Dependency,
) (*chart.Chart, error) {
	// Check if the dependency is already loaded.
	for _, includedDep := range parentChart.Dependencies() {
		if includedDep.Name() == dep.Name {
			return includedDep, nil
		}
	}

	if dep.Repository == "" {
		return nil, fmt.Errorf("chart dependency has no repository: %#v", dep)
	}

	pulledChart, err := c.client.Pull(ctx, dep.Name, dep.Repository, dep.Version, c.repos)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartPull, err)
	}

	depChart, err := loader.Load(pulledChart.path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartLoad, err)
	}

	return depChart, nil
}

// Normalize a chart name for file system use, that is, if chart name is
// foo/bar/baz, returns the last component as chart name.
func normalizeChartName(chartName string) string {
	strings.Join(strings.Split(chartName, "/"), "_")

	_, nc := path.Split(chartName)
	// We do not want to return the empty string or something else related to
	// filesystem access. Instead, return original string.
	if nc == "" || nc == "." || nc == ".." {
		return chartName
	}

	return nc
}

func removeSchemasFromObject(c *chart.Chart) {
	c.Schema = nil
	for _, d := range c.Dependencies() {
		removeSchemasFromObject(d)
	}
}
