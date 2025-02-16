package helm

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"time"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	helmkube "helm.sh/helm/v3/pkg/kube"

	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/kube"
)

type TemplateOpts struct {
	ValuesObject         map[string]any
	Proxy                string
	TargetRevision       string
	RepoURL              string
	ReleaseName          string
	Namespace            string
	ChartName            string
	KubeVersion          string
	NoProxy              string
	APIVersions          []string
	Timeout              time.Duration
	SkipCRDs             bool
	PassCredentials      bool
	SkipSchemaValidation bool
	SkipHooks            bool
}

type ChartClient interface {
	Pull(ctx context.Context, chartName, repoURL, targetRevision string, repos helmrepo.Getter) (string, error)
}

type Chart struct {
	Client       ChartClient
	Repos        helmrepo.Getter
	TemplateOpts *TemplateOpts
}

// NewChart creates a new [Chart].
func NewChart(client ChartClient, repos helmrepo.Getter, opts *TemplateOpts) (*Chart, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 60 * time.Second
	}

	return &Chart{
		Client:       client,
		Repos:        repos,
		TemplateOpts: opts,
	}, nil
}

// Template templates the Helm [Chart]. The [chart.Chart] and its dependencies
// are pulled as needed. The rendered output is then split into individual
// Kubernetes objects and returned as a slice of [unstructured.Unstructured].
func (c *Chart) Template(ctx context.Context) ([]*unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(ctx, c.TemplateOpts.Timeout)
	defer cancel()

	chartPath, err := c.Client.Pull(ctx,
		c.TemplateOpts.ChartName,
		c.TemplateOpts.RepoURL,
		c.TemplateOpts.TargetRevision,
		c.Repos,
	)
	if err != nil {
		return nil, fmt.Errorf("error pulling helm chart: %w", err)
	}

	out, err := c.templateData(ctx, chartPath)
	if err != nil {
		return nil, err
	}

	objs, err := kube.SplitYAML(out)
	if err != nil {
		return nil, fmt.Errorf("error parsing helm template output: %w", err)
	}

	return objs, nil
}

func (c *Chart) templateData(ctx context.Context, chartPath string) ([]byte, error) {
	var err error

	loadedChart, err := loader.Load(filepath.Clean(chartPath))
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %w", err)
	}

	// Keeping the schema in the charts will cause downstream templating to load
	// remote refs and validate against the schema, for the chart and all its
	// dependencies. This can be a massive and random-feeling performance hit,
	// and is largely unnecessary since KCL will be using the same, or a similar
	// schema to validate the values.
	if c.TemplateOpts.SkipSchemaValidation {
		removeSchemasFromObject(loadedChart)
	}

	// Recursively load and set all chart dependencies.
	workerCount := int64(runtime.GOMAXPROCS(0))
	sem := semaphore.NewWeighted(workerCount)
	if err := c.setChartDependencies(ctx, loadedChart, sem); err != nil {
		return nil, fmt.Errorf("failed to set chart dependencies: %w", err)
	}
	err = sem.Acquire(ctx, workerCount)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart dependencies: %w", err)
	}

	// Fail open instead of blocking the template.
	kv := &chartutil.KubeVersion{
		Major:   "999",
		Minor:   "999",
		Version: "v999.999.999",
	}
	if c.TemplateOpts.KubeVersion != "" {
		kv, err = chartutil.ParseKubeVersion(c.TemplateOpts.KubeVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse kube version: %w", err)
		}
	}

	av := chartutil.DefaultVersionSet
	if len(c.TemplateOpts.APIVersions) > 0 {
		av = c.TemplateOpts.APIVersions
	}

	ta := action.NewInstall(&action.Configuration{
		KubeClient: helmkube.New(genericclioptions.NewConfigFlags(false)),
		Capabilities: &chartutil.Capabilities{
			KubeVersion: *kv,
			APIVersions: av,
			HelmVersion: chartutil.DefaultCapabilities.HelmVersion,
		},
		Log: func(msg string, kv ...any) {
			slog.Debug(msg, kv...)
		},
	})
	ta.DryRun = true
	ta.DryRunOption = "client"
	ta.ClientOnly = true
	ta.DisableHooks = true
	ta.DisableOpenAPIValidation = c.TemplateOpts.SkipSchemaValidation
	ta.ReleaseName = c.TemplateOpts.ChartName
	ta.Namespace = c.TemplateOpts.Namespace
	ta.NameTemplate = c.TemplateOpts.ChartName
	ta.KubeVersion = kv
	ta.APIVersions = av

	// Set both, otherwise the defaults make things weird.
	ta.IncludeCRDs = !c.TemplateOpts.SkipCRDs
	ta.SkipCRDs = c.TemplateOpts.SkipCRDs

	if c.TemplateOpts.ValuesObject == nil {
		c.TemplateOpts.ValuesObject = make(map[string]any)
	}

	release, err := ta.Run(loadedChart, c.TemplateOpts.ValuesObject)
	if err != nil {
		return nil, fmt.Errorf("failed to run install action: %w", err)
	}

	manifest := release.Manifest

	if !c.TemplateOpts.SkipHooks {
		for _, hook := range release.Hooks {
			if hook == nil {
				continue
			}

			manifest += "\n---\n" + hook.Manifest
		}
	}

	return []byte(manifest), nil
}

// setChartDependencies concurrently loads and sets the dependencies of the
// target chart. It is called recursively until all dependencies are loaded.
// It uses a weighted semaphore to limit the number of concurrent loads.
func (c *Chart) setChartDependencies(ctx context.Context, target *chart.Chart, sem *semaphore.Weighted) error {
	loadedDeps := []*chart.Chart{}

	type loadResult struct {
		chart *chart.Chart
		err   error
	}

	depCount := len(target.Metadata.Dependencies)
	resultCh := make(chan loadResult, depCount)
	// The smaller semaphore of sem and innerSem will block.
	innerSem := semaphore.NewWeighted(int64(depCount))

	for _, chartDep := range target.Metadata.Dependencies {
		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to acquire semaphore: %w", err)
		}
		if err := innerSem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to acquire semaphore: %w", err)
		}
		go func() {
			defer sem.Release(1)
			defer innerSem.Release(1)

			dep, err := c.loadChartDependency(ctx, target, chartDep)
			if err != nil {
				resultCh <- loadResult{err: fmt.Errorf("failed to load chart dependency for %q: %w", target.Name(), err)}

				return
			}

			resultCh <- loadResult{chart: dep}
		}()
	}

	err := innerSem.Acquire(ctx, int64(depCount))
	if err != nil {
		return fmt.Errorf("failed to acquire semaphore: %w", err)
	}

	close(resultCh)

	var merr error
	for result := range resultCh {
		if result.err != nil {
			merr = multierror.Append(merr, result.err)

			continue
		}

		if err := c.setChartDependencies(ctx, result.chart, sem); err != nil {
			return fmt.Errorf("failed to load chart dependencies: %w", err)
		}

		loadedDeps = append(loadedDeps, result.chart)
	}
	if merr != nil {
		return fmt.Errorf("failed to load chart dependencies: %w", merr)
	}

	target.SetDependencies(loadedDeps...)

	return nil
}

func (c *Chart) loadChartDependency(ctx context.Context, parentChart *chart.Chart, dep *chart.Dependency) (*chart.Chart, error) {
	// Check if the dependency is already loaded.
	for _, includedDep := range parentChart.Dependencies() {
		if includedDep.Name() == dep.Name {
			return includedDep, nil
		}
	}

	if dep.Repository == "" {
		return nil, fmt.Errorf("dependency has no repository: %#v", dep)
	}

	depPath, err := c.Client.Pull(ctx, dep.Name, dep.Repository, dep.Version, c.Repos)
	if err != nil {
		return nil, fmt.Errorf("failed to pull dependency: %w", err)
	}

	depChart, err := loader.Load(depPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load dependency: %w", err)
	}

	if c.TemplateOpts.SkipSchemaValidation {
		removeSchemasFromObject(depChart)
	}

	return depChart, nil
}

func removeSchemasFromObject(c *chart.Chart) {
	c.Schema = nil
	for _, d := range c.Dependencies() {
		removeSchemasFromObject(d)
	}
}
