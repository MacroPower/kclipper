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

	"golang.org/x/sync/semaphore"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"k8s.io/apimachinery/pkg/api/resource"

	chart "helm.sh/helm/v4/pkg/chart/v2"

	"github.com/macropower/kclipper/pkg/helmrepo"
)

// ErrChartDependency indicates an error occurred while loading chart dependencies.
var ErrChartDependency = errors.New("chart dependency")

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
// clean up the extracted chart. If [PulledChart] references a directory,
// the path to the directory and a [NewNopCloser] is returned.
func (c *PulledChart) Extract(maxSize *resource.Quantity) (string, io.Closer, error) {
	closer := NewNopCloser()

	// If the chart is already extracted, return the path to the extracted chart.
	if dirExists(c.path) {
		return c.path, closer, nil
	}

	extractedPath, closer, err := c.extractChart(c.chart, c.path, maxSize)
	if err != nil {
		return "", nil, fmt.Errorf("decompress chart archive: %w", err)
	}

	return extractedPath, closer, nil
}

// Load will load the Helm chart into a [chart.Chart]. If [PulledChart]
// references a .tar.gz, it will be loaded directly into memory without
// extracting the files to disk. If [PulledChart] references a directory, the
// contents of the chart will be loaded from the filesystem. No closer is
// returned by this method, since no temporary files are created.
func (c *PulledChart) Load(ctx context.Context) (*chart.Chart, error) {
	loadedChart, err := loader.Load(c.path)
	if err != nil {
		return nil, fmt.Errorf("read chart from disk: %w", err)
	}

	// Recursively load and set all chart dependencies.
	err = c.loadChartDependencies(ctx, loadedChart)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartDependency, err)
	}

	return loadedChart, nil
}

func (c *PulledChart) extractChart(chartName, srcPath string, maxSize *resource.Quantity) (string, io.Closer, error) {
	tempDest, err := os.MkdirTemp("", "kclipper-*")
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
func (c *PulledChart) loadChartDependencies(ctx context.Context, target *chart.Chart) error {
	workerCount := int64(runtime.GOMAXPROCS(0))
	sem := semaphore.NewWeighted(workerCount)

	return c.setChartDependencies(ctx, target, sem)
}

func (c *PulledChart) setChartDependencies(
	ctx context.Context,
	target *chart.Chart,
	sem *semaphore.Weighted,
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
			return fmt.Errorf("acquire semaphore: %w", err)
		}

		err = innerSem.Acquire(ctx, 1)
		if err != nil {
			return fmt.Errorf("acquire semaphore: %w", err)
		}

		go func() {
			defer sem.Release(1)
			defer innerSem.Release(1)

			dep, err := c.getChartDependency(ctx, target, chartDep)
			if err != nil {
				resultCh <- loadResult{err: fmt.Errorf("get dependency %q: %w", target.Name(), err)}

				return
			}

			resultCh <- loadResult{chart: dep}
		}()
	}

	err := innerSem.Acquire(ctx, depCount)
	if err != nil {
		return fmt.Errorf("acquire semaphore: %w", err)
	}

	close(resultCh)

	var merr error

	for result := range resultCh {
		if result.err != nil {
			merr = errors.Join(merr, result.err)

			continue
		}

		err := c.setChartDependencies(ctx, result.chart, sem)
		if err != nil {
			return fmt.Errorf("set chart dependencies: %w", err)
		}

		loadedDeps = append(loadedDeps, result.chart)
	}

	if merr != nil {
		return fmt.Errorf("load chart dependencies: %w", merr)
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
		return nil, fmt.Errorf("load chart dependency: %w", err)
	}

	return depChart, nil
}

// Normalize a chart name for file system use, that is, if chart name is
// foo/bar/baz, returns the last component as chart name.
func normalizeChartName(chartName string) string {
	_, nc := path.Split(chartName)
	// We do not want to return the empty string or something else related to
	// filesystem access. Instead, return original string.
	if nc == "" || nc == "." || nc == ".." {
		return chartName
	}

	return nc
}
