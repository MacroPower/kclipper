package helm

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/registry"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/MacroPower/kclipper/pkg/helmrepo"
)

// A thin wrapper around helm.sh/helm, adding logging and error translation.
type Cmd struct {
	rc        *registry.Client
	settings  *cli.EnvSettings
	helmHome  string
	WorkDir   string
	proxy     string
	noProxy   string
	IsLocal   bool
	IsHelmOci bool
}

func NewCmdWithVersion(workDir, proxy, noProxy string) (*Cmd, error) {
	rc, err := registry.NewClient(registry.ClientOptEnableCache(true))
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory for helm: %w", err)
	}

	return &Cmd{
		WorkDir:  workDir,
		helmHome: tmpDir,
		proxy:    proxy,
		noProxy:  noProxy,
		rc:       rc,
		settings: cli.New(),
	}, nil
}

func (c *Cmd) Fetch(chart, version, destination string, repo *helmrepo.Repo) (string, error) {
	ap := action.NewPullWithOpts(action.WithConfig(&action.Configuration{
		RegistryClient: c.rc,
	}))
	ap.Settings = c.settings
	ap.Untar = false
	ap.DestDir = destination

	if version != "" {
		ap.Version = version
	}

	if repo != nil {
		ap.RepoURL = repo.URL.String()
		ap.Username = repo.Username
		ap.Password = repo.Password
		ap.CaFile = repo.CAPath.String()
		ap.CertFile = repo.TLSClientCertDataPath.String()
		ap.KeyFile = repo.TLSClientCertKeyPath.String()
		ap.PassCredentialsAll = repo.PassCredentials
		ap.InsecureSkipTLSverify = repo.InsecureSkipVerify
	}

	out, err := ap.Run(chart)
	if err != nil {
		return "", fmt.Errorf("failed to fetch chart: %w", err)
	}

	return out, nil
}

func (c *Cmd) Template(chartPath string, opts *CmdTemplateOpts) (string, string, error) {
	// Fail open instead of blocking the template.
	kv := &chartutil.KubeVersion{
		Major:   "999",
		Minor:   "999",
		Version: "v999.999.999",
	}

	var err error

	if opts.KubeVersion != "" {
		kv, err = chartutil.ParseKubeVersion(opts.KubeVersion)
		if err != nil {
			return "", "", fmt.Errorf("failed to parse kube version: %w", err)
		}
	}

	av := chartutil.DefaultVersionSet
	if len(opts.APIVersions) > 0 {
		av = opts.APIVersions
	}

	loadedChart, err := loader.Load(filepath.Clean(path.Join(c.WorkDir, chartPath)))
	if err != nil {
		return "", "", fmt.Errorf("failed to load chart: %w", err)
	}
	// Keeping the schema in the charts will cause downstream templating to load
	// remote refs and validate against the schema, for the chart and all its
	// dependencies. This can be a massive and random-feeling performance hit,
	// and is largely unnecessary since KCL will be using the same, or a similar
	// schema to validate the values.
	if opts.SkipSchemaValidation {
		removeSchemasFromObject(loadedChart)
	}

	loadedDeps := []*chart.Chart{}

	for _, chartDep := range loadedChart.Metadata.Dependencies {
		isLoaded := false

		for _, includedDeps := range loadedChart.Dependencies() {
			if includedDeps.Name() == chartDep.Name {
				loadedDeps = append(loadedDeps, includedDeps)
				isLoaded = true

				break
			}
		}

		if isLoaded {
			continue
		}

		if chartDep.Repository == "" {
			return "", "", fmt.Errorf("dependency has no repository: %#v", chartDep)
		}

		depPath, err := opts.DependencyPuller.Pull(chartDep.Name, chartDep.Repository, chartDep.Version, opts.RepoGetter)
		if err != nil {
			return "", "", fmt.Errorf("failed to pull dependency: %w", err)
		}

		dep, err := loader.Load(depPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to load dependency: %w", err)
		}

		if opts.SkipSchemaValidation {
			removeSchemasFromObject(dep)
		}

		loadedDeps = append(loadedDeps, dep)
	}

	loadedChart.SetDependencies(loadedDeps...)

	ta := action.NewInstall(&action.Configuration{
		KubeClient:     kube.New(genericclioptions.NewConfigFlags(false)),
		RegistryClient: c.rc,
		Capabilities: &chartutil.Capabilities{
			KubeVersion: *kv,
			APIVersions: av,
			HelmVersion: chartutil.DefaultCapabilities.HelmVersion,
		},
	})
	ta.DryRun = true
	ta.DryRunOption = "client"
	ta.ClientOnly = true
	ta.DisableHooks = true
	ta.DisableOpenAPIValidation = opts.SkipSchemaValidation
	ta.ReleaseName = opts.Name
	ta.Namespace = opts.Namespace
	ta.NameTemplate = opts.Name
	ta.KubeVersion = kv
	ta.APIVersions = av

	// Set both, otherwise the defaults make things weird.
	ta.IncludeCRDs = !opts.SkipCrds
	ta.SkipCRDs = opts.SkipCrds

	if opts.Values == nil {
		opts.Values = make(map[string]any)
	}

	release, err := ta.Run(loadedChart, opts.Values)
	if err != nil {
		return "", "", fmt.Errorf("failed to run install action: %w", err)
	}

	manifest := release.Manifest

	if !opts.SkipHooks {
		for _, hook := range release.Hooks {
			if hook == nil {
				continue
			}

			manifest += "\n---\n" + hook.Manifest
		}
	}

	return manifest, release.Name, nil
}

func removeSchemasFromObject(chart *chart.Chart) {
	chart.Schema = nil
	for _, d := range chart.Dependencies() {
		removeSchemasFromObject(d)
	}
}

type CmdTemplateOpts struct {
	RepoGetter           helmrepo.Getter
	DependencyPuller     ChartClient
	Values               map[string]any
	Name                 string
	Namespace            string
	KubeVersion          string
	APIVersions          []string
	SkipCrds             bool
	SkipSchemaValidation bool
	SkipHooks            bool
}
