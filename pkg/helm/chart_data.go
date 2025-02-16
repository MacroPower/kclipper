package helm

import (
	"fmt"
	"log/slog"
	"path/filepath"

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
	SkipCRDs             bool
	PassCredentials      bool
	SkipSchemaValidation bool
	SkipHooks            bool
}

type ChartClient interface {
	Pull(chartName, repoURL, targetRevision string, repos helmrepo.Getter) (string, error)
}

type Chart struct {
	Client       ChartClient
	Repos        helmrepo.Getter
	TemplateOpts *TemplateOpts
	path         string
}

func NewChart(client ChartClient, repos helmrepo.Getter, opts *TemplateOpts) (*Chart, error) {
	chartPath, err := client.Pull(opts.ChartName, opts.RepoURL, opts.TargetRevision, repos)
	if err != nil {
		return nil, fmt.Errorf("error pulling helm chart: %w", err)
	}

	return &Chart{
		Client:       client,
		Repos:        repos,
		TemplateOpts: opts,
		path:         chartPath,
	}, nil
}

// Template templates the Helm [Chart]. The rendered output is then split into
// individual Kubernetes objects and returned as a slice of
// [unstructured.Unstructured] objects.
func (c *Chart) Template() ([]*unstructured.Unstructured, error) {
	out, err := c.templateData()
	if err != nil {
		return nil, err
	}

	objs, err := kube.SplitYAML(out)
	if err != nil {
		return nil, fmt.Errorf("error parsing helm template output: %w", err)
	}

	return objs, nil
}

func (c *Chart) templateData() ([]byte, error) {
	var err error

	loadedChart, err := loader.Load(filepath.Clean(c.path))
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
	if err := c.setChartDependencies(loadedChart); err != nil {
		return nil, fmt.Errorf("failed to set chart dependencies: %w", err)
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

func (c *Chart) setChartDependencies(parentChart *chart.Chart) error {
	loadedDeps := []*chart.Chart{}

	for _, chartDep := range parentChart.Metadata.Dependencies {
		loadedDep, err := c.loadChartDependency(parentChart, chartDep)
		if err != nil {
			return fmt.Errorf("failed to load dependency: %w", err)
		}

		for _, dep := range loadedDep.Dependencies() {
			if err := c.setChartDependencies(dep); err != nil {
				return fmt.Errorf("failed to set chart dependencies: %w", err)
			}
		}

		loadedDeps = append(loadedDeps, loadedDep)
	}

	parentChart.SetDependencies(loadedDeps...)

	return nil
}

func (c *Chart) loadChartDependency(parentChart *chart.Chart, dep *chart.Dependency) (*chart.Chart, error) {
	// Check if the dependency is already loaded.
	for _, includedDep := range parentChart.Dependencies() {
		if includedDep.Name() == dep.Name {
			return includedDep, nil
		}
	}

	if dep.Repository == "" {
		return nil, fmt.Errorf("dependency has no repository: %#v", dep)
	}

	depPath, err := c.Client.Pull(dep.Name, dep.Repository, dep.Version, c.Repos)
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
