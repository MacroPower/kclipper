package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/macropower/kclipper/pkg/helmrepo"
	"github.com/macropower/kclipper/pkg/paths"
	"github.com/macropower/kclipper/pkg/syncs"
)

var (
	globalLock = syncs.NewKeyLock()

	// DefaultClient is a [Client] configured with default paths and the
	// ARGOCD_APP_PROJECT_NAME environment variable.
	DefaultClient = MustNewClient(
		paths.NewStaticTempPaths(filepath.Join(os.TempDir(), "charts"), paths.NewBase64PathEncoder()),
		os.Getenv("ARGOCD_APP_PROJECT_NAME"),
	)
)

// PathCacher stores and retrieves filesystem paths by key.
// See [paths.StaticTempPaths] for an implementation.
type PathCacher interface {
	Add(key, value string)
	GetPath(key string) (string, error)
	GetPathIfExists(key string) string
	GetPaths() map[string]string
}

// ChartClient pulls Helm charts and returns the result.
// See [Client] for an implementation.
type ChartClient interface {
	Pull(ctx context.Context, chartName, repoURL, targetRevision string, repos helmrepo.Getter) (*PulledChart, error)
}

// Client pulls and caches Helm charts from local and remote repositories.
// Create instances with [NewClient] or [MustNewClient].
type Client struct {
	Paths          PathCacher
	RepoLock       syncs.KeyLocker
	MaxExtractSize resource.Quantity
	rc             *registry.Client
	helmHome       string
	Project        string
	Proxy          string
	NoProxy        string
}

// NewClient creates a new [Client].
func NewClient(pc PathCacher, project string) (*Client, error) {
	rc, err := registry.NewClient(registry.ClientOptEnableCache(true))
	if err != nil {
		return nil, fmt.Errorf("create registry client: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return nil, fmt.Errorf("create temporary directory for helm: %w", err)
	}

	return &Client{
		Paths:    pc,
		RepoLock: globalLock,
		rc:       rc,
		helmHome: tmpDir,
		Project:  project,
	}, nil
}

// MustNewClient runs [NewClient] and panics on any errors.
func MustNewClient(pc PathCacher, project string) *Client {
	c, err := NewClient(pc, project)
	if err != nil {
		panic(err)
	}

	return c
}

// Pull pulls the Helm chart and returns the path to the chart directory or
// .tar.gz file. Pulled charts will be stored in the injected [PathCacher], and
// subsequent requests will try to use [PathCacher] rather than re-pulling the
// chart.
func (c *Client) Pull(ctx context.Context, chart, repo, version string, repos helmrepo.Getter) (*PulledChart, error) {
	hr, err := repos.Get(repo)
	if err != nil {
		return nil, fmt.Errorf("get repo: %q: %w", repo, err)
	}

	pc := &PulledChart{
		chart:  chart,
		repos:  repos,
		client: c,
	}

	if hr.IsLocal() {
		chartPath, err := c.getLocalChart(chart, hr)
		if err != nil {
			return nil, fmt.Errorf("get local chart: %w", err)
		}

		pc.path = chartPath

		return pc, err
	}

	chartPath, err := c.getCachedOrRemoteChart(ctx, chart, version, hr)
	if err != nil {
		return nil, fmt.Errorf("get cached or remote chart: %w", err)
	}

	pc.path = chartPath

	return pc, nil
}

// CleanChartCache removes the cached chart directory for the given chart.
func (c *Client) CleanChartCache(chart, repo, version string) error {
	cachePath, err := c.getCachedChartPath(chart, repo, version)
	if err != nil {
		return fmt.Errorf("get cached chart path: %w", err)
	}

	err = os.RemoveAll(cachePath)
	if err != nil {
		return fmt.Errorf("remove chart cache at %q: %w", cachePath, err)
	}

	return nil
}

func (c *Client) getCachedChartPath(chart, repo, version string) (string, error) {
	keyData, err := json.Marshal(
		map[string]string{"url": repo, "chart": chart, "version": version, "project": c.Project},
	)
	if err != nil {
		return "", fmt.Errorf("marshal key data: %w", err)
	}

	chartPath, err := c.Paths.GetPath(string(keyData))
	if err != nil {
		return "", fmt.Errorf("get path: %w", err)
	}

	return chartPath, nil
}

func (c *Client) getLocalChart(chart string, repo *helmrepo.Repo) (string, error) {
	chartPath := filepath.Join(repo.URL.String(), chart)
	if !dirExists(chartPath) {
		return "", fmt.Errorf("chart directory does not exist: %q", chartPath)
	}

	return chartPath, nil
}

func (c *Client) getCachedOrRemoteChart(
	ctx context.Context,
	chart, version string,
	repo *helmrepo.Repo,
) (string, error) {
	cachedChartPath, err := c.getCachedChartPath(chart, repo.URL.String(), version)
	if err != nil {
		return "", fmt.Errorf("get cached chart path: %w", err)
	}

	c.RepoLock.Lock(cachedChartPath)
	defer c.RepoLock.Unlock(cachedChartPath)

	// Check if chart tar is already downloaded.
	exists, err := fileExists(cachedChartPath)
	if err != nil {
		return "", fmt.Errorf("check cached chart path: %w", err)
	}

	if !exists {
		err := c.pullRemoteChart(ctx, chart, version, cachedChartPath, repo)
		if err != nil {
			return "", fmt.Errorf("pull remote chart: %w", err)
		}
	}

	return cachedChartPath, nil
}

func (c *Client) pullRemoteChart(ctx context.Context, chart, version, dstPath string, repo *helmrepo.Repo) error {
	// Create empty temp directory to extract chart from the registry.
	tempDest, err := os.MkdirTemp("", "kclipper-*")
	if err != nil {
		return fmt.Errorf("create temporary destination directory: %w", err)
	}

	defer func() { _ = os.RemoveAll(tempDest) }()

	logger := slog.With(
		slog.String("chart", chart),
	)

	ap := action.NewPullWithOpts(action.WithConfig(&action.Configuration{
		RegistryClient: c.rc,
		Log: func(msg string, kv ...any) {
			slog.Debug(msg, kv...)
		},
	}))
	ap.Settings = &cli.EnvSettings{
		RepositoryCache: filepath.Join(c.helmHome, "cache"),
	}
	ap.Untar = false
	ap.DestDir = tempDest

	if version != "" {
		ap.Version = version
	}

	if repo != nil {
		if u, ok := repo.URL.URL(); ok {
			if u.Scheme == "oci" {
				chart = repo.URL.String()
			} else {
				ap.RepoURL = repo.URL.String()
			}
		}

		ap.Username = repo.Username
		ap.Password = repo.Password
		ap.CaFile = repo.CAPath.String()
		ap.CertFile = repo.TLSClientCertDataPath.String()
		ap.KeyFile = repo.TLSClientCertKeyPath.String()
		ap.PassCredentialsAll = repo.PassCredentials
		ap.InsecureSkipTLSverify = repo.InsecureSkipVerify
	}

	logger.InfoContext(ctx, "pulling chart",
		slog.String("chart_ref", chart),
		slog.String("version", ap.Version),
		slog.String("destination", ap.DestDir),
		slog.String("repo_url", ap.RepoURL),
		slog.Bool("insecure_skip_tls_verify", ap.InsecureSkipTLSverify),
		slog.Bool("pass_credentials_all", ap.PassCredentialsAll),
	)

	done := make(chan error, 1)
	go func() {
		_, err = ap.Run(chart)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("execute helm pull: %w", ctx.Err())
	case err := <-done:
		if err != nil {
			return fmt.Errorf("execute helm pull: %w", err)
		}
	}

	logger.DebugContext(ctx, "chart pull complete")

	// 'helm pull/fetch' file downloads chart into the tgz file and we move that
	// to where we want it, if the pull was successful.
	infos, err := os.ReadDir(tempDest)
	if err != nil {
		return fmt.Errorf("read directory %q: %w", tempDest, err)
	}

	if len(infos) != 1 {
		return fmt.Errorf("expected 1 file, found %v", len(infos))
	}

	chartFilePath := filepath.Join(tempDest, infos[0].Name())
	logger.DebugContext(ctx, "moving pulled chart",
		slog.String("src", chartFilePath),
		slog.String("dst", dstPath),
	)

	err = os.Rename(chartFilePath, dstPath)
	if err != nil {
		return fmt.Errorf("rename file from %q to %q: %w", chartFilePath, dstPath, err)
	}

	return nil
}
