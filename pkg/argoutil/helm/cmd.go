package helm

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/registry"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// A thin wrapper around helm.sh/helm, adding logging and error translation.
type Cmd struct {
	helmHome  string
	WorkDir   string
	IsLocal   bool
	IsHelmOci bool
	proxy     string
	noProxy   string

	rc       *registry.Client
	settings *cli.EnvSettings

	reposByName map[string]Repo
	reposByURL  map[string]Repo
}

type Repo struct {
	Name  string
	URL   string
	Creds Creds
}

func NewCmd(workDir string, version string, proxy string, noProxy string) (*Cmd, error) {
	switch version {
	// If v3 is specified (or by default, if no value is specified) then use v3
	case "", "v3":
		return NewCmdWithVersion(workDir, false, proxy, noProxy)
	}
	return nil, fmt.Errorf("helm chart version '%s' is not supported", version)
}

func NewCmdWithVersion(workDir string, isHelmOci bool, proxy string, noProxy string) (*Cmd, error) {
	rc, err := registry.NewClient(registry.ClientOptEnableCache(true))
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory for helm: %w", err)
	}
	return &Cmd{
		WorkDir:     workDir,
		helmHome:    tmpDir,
		IsHelmOci:   isHelmOci,
		proxy:       proxy,
		noProxy:     noProxy,
		rc:          rc,
		settings:    cli.New(),
		reposByName: map[string]Repo{},
		reposByURL:  map[string]Repo{},
	}, nil
}

func (c *Cmd) RegistryLogin(repo string, creds Creds) (string, error) {
	opts := []registry.LoginOption{}

	if creds.Username != "" && creds.Password != "" {
		opts = append(opts, registry.LoginOptBasicAuth(creds.Username, creds.Password))
	}

	if creds.CAPath != "" && len(creds.CertData) > 0 && len(creds.KeyData) > 0 {
		certPath, closer, err := writeToTmp(creds.CertData)
		if err != nil {
			return "", fmt.Errorf("failed to write certificate data to temporary file: %w", err)
		}
		defer tryClose(closer)

		keyPath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", fmt.Errorf("failed to write key data to temporary file: %w", err)
		}
		defer tryClose(closer)

		opts = append(opts, registry.LoginOptTLSClientConfig(certPath, keyPath, creds.CAPath))
	}

	if creds.InsecureSkipVerify {
		opts = append(opts, registry.LoginOptInsecure(true))
	}

	err := c.rc.Login(repo, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to login to registry: %w", err)
	}
	return "ok", nil
}

func (c *Cmd) RegistryLogout(repo string, creds Creds) (string, error) {
	err := c.rc.Logout(repo)
	if err != nil {
		return "", fmt.Errorf("failed to logout from registry: %w", err)
	}
	return "ok", nil
}

func (c *Cmd) RepoAdd(name string, url string, creds Creds, passCredentials bool) (string, error) {
	r := Repo{
		Name:  name,
		URL:   url,
		Creds: creds,
	}
	c.reposByName[name] = r
	c.reposByURL[url] = r

	return "ok", nil
}

func (c *Cmd) Fetch(repo, chartName, version, destination string, creds Creds, passCredentials bool) (string, error) {
	ap := action.NewPullWithOpts(action.WithConfig(&action.Configuration{
		RegistryClient: c.rc,
	}))
	ap.Settings = c.settings

	ap.Untar = false

	ap.DestDir = destination
	if version != "" {
		ap.Version = version
	}
	if creds.Username != "" {
		ap.Username = creds.Username
	}
	if creds.Password != "" {
		ap.Password = creds.Password
	}
	if creds.InsecureSkipVerify {
		ap.InsecureSkipTLSverify = true
	}

	ap.RepoURL = repo

	if creds.CAPath != "" {
		ap.CaFile = creds.CAPath
	}
	if len(creds.CertData) > 0 {
		filePath, closer, err := writeToTmp(creds.CertData)
		if err != nil {
			return "", fmt.Errorf("failed to write certificate data to temporary file: %w", err)
		}
		defer tryClose(closer)
		ap.CertFile = filePath
	}
	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", fmt.Errorf("failed to write key data to temporary file: %w", err)
		}
		defer tryClose(closer)
		ap.KeyFile = filePath
	}
	if passCredentials {
		ap.PassCredentialsAll = true
	}

	out, err := ap.Run(chartName)
	if err != nil {
		return "", fmt.Errorf("failed to fetch chart: %w", err)
	}
	return out, nil
}

func (c *Cmd) PullOCI(repo string, chart string, version string, destination string, creds Creds) (string, error) {
	repoURL := fmt.Sprintf("oci://%s/%s", repo, chart)
	out, err := c.Fetch(repoURL, chart, version, destination, creds, false)
	if err != nil {
		return "", fmt.Errorf("failed to pull OCI chart: %w", err)
	}
	return out, nil
}

func (c *Cmd) dependencyBuild() (string, error) {
	// out, _, err := c.run("dependency", "build")
	// if err != nil {
	// 	return "", fmt.Errorf("failed to build dependencies: %w", err)
	// }
	return "ok", nil
}

func (c *Cmd) template(chartPath string, opts *TemplateOpts) (string, string, error) {
	// if callback, err := cleanupChartLockFile(filepath.Clean(path.Join(c.WorkDir, chartPath))); err == nil {
	// 	defer callback()
	// } else {
	// 	return "", "", fmt.Errorf("failed to clean up chart lock file: %w", err)
	// }

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

		depRepo := Repo{
			URL: chartDep.Repository,
		}
		if strings.HasPrefix(chartDep.Repository, "@") {
			if dr, ok := c.reposByName[chartDep.Repository]; ok {
				depRepo = dr
			}
		} else {
			if dr, ok := c.reposByURL[chartDep.Repository]; ok {
				depRepo = dr
			}
		}

		if depRepo.URL == "" {
			return "", "", fmt.Errorf("dependency has no repository: %#v", chartDep)
		}

		depClient := NewClient(depRepo.URL, depRepo.Creds, false, "", "")
		depPath, err := depClient.PullChart(chartDep.Name, chartDep.Version, "", false, 0, true)
		if err != nil {
			return "", "", fmt.Errorf("failed to pull chart dependency: %w", err)
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
	ta.DisableOpenAPIValidation = true
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

	return release.Manifest, release.Name, nil
}

func (c *Cmd) Close() {
	_ = os.RemoveAll(c.helmHome)
}

func removeSchemasFromObject(chart *chart.Chart) {
	chart.Schema = nil
	for _, d := range chart.Dependencies() {
		removeSchemasFromObject(d)
	}
}

func writeToTmp(data []byte) (string, *InlineCloser, error) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	err = os.WriteFile(file.Name(), data, 0o600)
	if err != nil {
		_ = os.RemoveAll(file.Name())
		return "", nil, fmt.Errorf("failed to write data to temporary file: %w", err)
	}
	defer func() {
		if err = file.Close(); err != nil {
			slog.Error("error closing file", "file", file.Name(), "security", 2, "CWE", 755, "err", err)
		}
	}()
	return file.Name(), newInlineCloser(func() error {
		return os.RemoveAll(file.Name())
	}), nil
}

type TemplateOpts struct {
	Name        string
	Namespace   string
	KubeVersion string
	APIVersions []string
	Values      map[string]any
	SkipCrds    bool

	SkipSchemaValidation bool
}

// // Workaround for Helm3 behavior (see https://github.com/helm/helm/issues/6870).
// // The `helm template` command generates Chart.lock after which `helm dependency build` does not work.
// // As workaround removing lock file unless it exists before running helm template.
// func cleanupChartLockFile(chartPath string) (func(), error) {
// 	exists := true
// 	lockPath := path.Join(chartPath, "Chart.lock")
// 	if _, err := os.Stat(lockPath); err != nil {
// 		if os.IsNotExist(err) {
// 			exists = false
// 		} else {
// 			return nil, fmt.Errorf("failed to check lock file status: %w", err)
// 		}
// 	}
// 	return func() {
// 		if !exists {
// 			_ = os.Remove(lockPath)
// 		}
// 	}, nil
// }
