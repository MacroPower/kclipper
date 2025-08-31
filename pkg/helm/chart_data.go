package helm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	helmkube "helm.sh/helm/v3/pkg/kube"

	"github.com/macropower/kclipper/pkg/helmrepo"
	"github.com/macropower/kclipper/pkg/kube"
)

var (
	ErrChartPull          = errors.New("error pulling chart")
	ErrChartTemplate      = errors.New("error templating chart")
	ErrChartLoad          = errors.New("error loading chart")
	ErrChartTemplateParse = errors.New("error parsing chart template output")
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

type Chart struct {
	Client       ChartClient
	Repos        helmrepo.Getter
	TemplateOpts *TemplateOpts
}

// NewChart creates a new [Chart].
func NewChart(client ChartClient, repos helmrepo.Getter, opts *TemplateOpts) (*Chart, error) {
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

	loadedChart, err := pulledChart.Load(ctx, c.TemplateOpts.SkipSchemaValidation)
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
	kv := &chartutil.KubeVersion{
		Major:   "1",
		Minor:   "999",
		Version: "v1.999.999",
	}
	if t.KubeVersion != "" {
		kv, err = chartutil.ParseKubeVersion(t.KubeVersion)
		if err != nil {
			return nil, fmt.Errorf("parse kube version: %w", err)
		}
	}

	av := chartutil.DefaultVersionSet
	if len(t.APIVersions) > 0 {
		av = t.APIVersions
	}

	ta := action.NewInstall(&action.Configuration{
		KubeClient: helmkube.New(nil),
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
	ta.DisableOpenAPIValidation = t.SkipSchemaValidation
	ta.ReleaseName = t.ChartName
	ta.Namespace = t.Namespace
	ta.NameTemplate = t.ChartName
	ta.KubeVersion = kv
	ta.APIVersions = av

	// Set both, otherwise the defaults make things weird.
	ta.IncludeCRDs = !t.SkipCRDs
	ta.SkipCRDs = t.SkipCRDs

	if t.ValuesObject == nil {
		t.ValuesObject = make(map[string]any)
	}

	release, err := ta.RunWithContext(ctx, loadedChart, t.ValuesObject)
	if err != nil {
		return nil, fmt.Errorf("execute helm install: %w", err)
	}

	manifest := release.Manifest

	if !t.SkipHooks {
		for _, hook := range release.Hooks {
			if hook == nil {
				continue
			}

			manifest += "\n---\n" + hook.Manifest
		}
	}

	return []byte(manifest), nil
}
