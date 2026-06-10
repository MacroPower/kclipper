package helm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	release "helm.sh/helm/v4/pkg/release/v1"

	"github.com/macropower/kclipper/pkg/helmrepo"
	"github.com/macropower/kclipper/pkg/kube"
)

var (
	// ErrChartPull indicates an error occurred while pulling a chart.
	ErrChartPull = errors.New("pull chart")

	// ErrChartTemplate indicates an error occurred while templating a chart.
	ErrChartTemplate = errors.New("template chart")

	// ErrChartLoad indicates an error occurred while loading a chart.
	ErrChartLoad = errors.New("load chart")

	// ErrChartTemplateParse indicates an error occurred while parsing chart template output.
	ErrChartTemplateParse = errors.New("parse chart template output")
)

// TemplateOpts configures Helm chart template rendering.
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

// Chart renders Helm chart templates into Kubernetes resources.
// Create instances with [NewChart].
type Chart struct {
	Client       ChartClient
	Repos        helmrepo.Getter
	TemplateOpts *TemplateOpts
}

// NewChart creates a new [Chart].
func NewChart(client ChartClient, repos helmrepo.Getter, opts *TemplateOpts) *Chart {
	return &Chart{
		Client:       client,
		Repos:        repos,
		TemplateOpts: opts,
	}
}

// Template templates the Helm [Chart]. The [chart.Chart] and its dependencies
// are pulled as needed. The rendered output is then split into individual
// Kubernetes resources and returned as a slice of [kube.Object].
func (c *Chart) Template(ctx context.Context) ([]kube.Object, error) {
	cancel := func() {}
	if c.TemplateOpts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.TemplateOpts.Timeout)
	}

	defer cancel()

	pulledChart, err := c.Client.Pull(ctx,
		c.TemplateOpts.ChartName,
		c.TemplateOpts.RepoURL,
		c.TemplateOpts.TargetRevision,
		c.Repos,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartPull, err)
	}

	loadedChart, err := pulledChart.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartLoad, err)
	}

	out, err := templateData(ctx, loadedChart, c.TemplateOpts)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartTemplate, err)
	}

	objs, err := kube.SplitYAML(out)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartTemplateParse, err)
	}

	return objs, nil
}

func templateData(ctx context.Context, loadedChart *chart.Chart, t *TemplateOpts) ([]byte, error) {
	var err error

	// Fail open instead of blocking the template.
	kv := &common.KubeVersion{
		Major:   "1",
		Minor:   "999",
		Version: "v1.999.999",
	}
	if t.KubeVersion != "" {
		kv, err = common.ParseKubeVersion(t.KubeVersion)
		if err != nil {
			return nil, fmt.Errorf("parse kube version: %w", err)
		}
	}

	cfg := &action.Configuration{}
	cfg.SetLogger(newDebugHandler())

	ta := action.NewInstall(cfg)
	// In client-only dry-run mode, Helm substitutes mock capabilities, kube
	// client, and release storage, applying KubeVersion and appending
	// APIVersions to the default version set.
	ta.DryRunStrategy = action.DryRunClient
	ta.DisableHooks = true
	// Validating values against chart schemas can cause remote JSON Schema
	// refs to be loaded for the chart and all of its dependencies. This can be
	// a massive and random-feeling performance hit, and is largely unnecessary
	// since KCL will be using the same, or a similar schema to validate the
	// values.
	ta.SkipSchemaValidation = t.SkipSchemaValidation
	ta.ReleaseName = t.ReleaseName
	if ta.ReleaseName == "" {
		ta.ReleaseName = t.ChartName
	}

	ta.Namespace = t.Namespace
	ta.KubeVersion = kv
	ta.APIVersions = common.VersionSet(t.APIVersions)

	// Set both, otherwise the defaults make things weird.
	ta.IncludeCRDs = !t.SkipCRDs
	ta.SkipCRDs = t.SkipCRDs

	if t.ValuesObject == nil {
		t.ValuesObject = make(map[string]any)
	}

	releaser, err := ta.RunWithContext(ctx, loadedChart, t.ValuesObject)
	if err != nil {
		return nil, fmt.Errorf("execute helm install: %w", err)
	}

	rel, ok := releaser.(*release.Release)
	if !ok {
		return nil, fmt.Errorf("unexpected release type: %T", releaser)
	}

	manifests := bytes.NewBufferString(rel.Manifest)
	if !t.SkipHooks {
		for _, hook := range rel.Hooks {
			if hook == nil {
				continue
			}

			manifests.WriteString("\n---\n" + hook.Manifest)
		}
	}

	return manifests.Bytes(), nil
}
