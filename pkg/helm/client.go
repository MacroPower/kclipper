package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/registry"

	chartrepo "helm.sh/helm/v4/pkg/repo/v1"

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
	Paths     PathCacher
	RepoLock  syncs.KeyLocker
	rc        *registry.Client
	transport *http.Transport
	helmHome  string
	Project   string
	Proxy     string
	NoProxy   string
}

// ClientOption configures a [Client].
//
// Available options:
//   - [WithProxy]
type ClientOption func(*Client)

// WithProxy returns a [ClientOption] that routes chart downloads through the
// given HTTP(S) proxy URL. Hosts matching the comma-separated noProxy list
// bypass the proxy. When unset, proxy environment variables are honored.
func WithProxy(proxy, noProxy string) ClientOption {
	return func(c *Client) {
		c.Proxy = proxy
		c.NoProxy = noProxy
	}
}

// NewClient creates a new [Client].
func NewClient(pc PathCacher, project string, opts ...ClientOption) (*Client, error) {
	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return nil, fmt.Errorf("create temporary directory for helm: %w", err)
	}

	c := &Client{
		Paths:    pc,
		RepoLock: globalLock,
		helmHome: tmpDir,
		Project:  project,
	}
	for _, opt := range opts {
		opt(c)
	}

	c.transport = c.proxyTransport()

	rcOpts := []registry.ClientOption{registry.ClientOptEnableCache(true)}
	if c.transport != nil {
		rcOpts = append(rcOpts, registry.ClientOptHTTPClient(&http.Client{Transport: c.transport}))
	}

	rc, err := registry.NewClient(rcOpts...)
	if err != nil {
		return nil, fmt.Errorf("create registry client: %w", err)
	}

	c.rc = rc

	return c, nil
}

// MustNewClient runs [NewClient] and panics on any errors.
func MustNewClient(pc PathCacher, project string, opts ...ClientOption) *Client {
	c, err := NewClient(pc, project, opts...)
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
	// Create empty temp directory to download the chart into.
	tempDest, err := os.MkdirTemp("", "kclipper-*")
	if err != nil {
		return fmt.Errorf("create temporary destination directory: %w", err)
	}

	defer func() { _ = os.RemoveAll(tempDest) }()

	logger := slog.With(
		slog.String("chart", chart),
	)

	chartRef := chart

	var (
		repoURL            string
		username           string
		password           string
		caFile             string
		certFile           string
		keyFile            string
		insecureSkipVerify bool
		passCredentials    bool
	)

	if repo != nil {
		if u, ok := repo.URL.URL(); ok {
			if u.Scheme == "oci" {
				chartRef = repo.URL.String()
			} else {
				repoURL = repo.URL.String()
			}
		}

		username = repo.Username
		password = repo.Password
		caFile = repo.CAPath.String()
		certFile = repo.TLSClientCertDataPath.String()
		keyFile = repo.TLSClientCertKeyPath.String()
		insecureSkipVerify = repo.InsecureSkipVerify
		passCredentials = repo.PassCredentials
	}

	getters, err := c.getters(certFile, keyFile, caFile, insecureSkipVerify)
	if err != nil {
		return fmt.Errorf("create getters: %w", err)
	}

	dl := &downloader.ChartDownloader{
		Out:     io.Discard,
		Verify:  downloader.VerifyNever,
		Getters: getters,
		Options: []getter.Option{
			getter.WithBasicAuth(username, password),
			getter.WithPassCredentialsAll(passCredentials),
			getter.WithTLSClientConfig(certFile, keyFile, caFile),
			getter.WithInsecureSkipVerifyTLS(insecureSkipVerify),
			getter.WithRegistryClient(c.rc),
		},
		RegistryClient: c.rc,
		ContentCache:   filepath.Join(c.helmHome, "content"),
	}

	logger.InfoContext(ctx, "pulling chart",
		slog.String("chart_ref", chartRef),
		slog.String("version", version),
		slog.String("destination", tempDest),
		slog.String("repo_url", repoURL),
		slog.Bool("insecure_skip_tls_verify", insecureSkipVerify),
		slog.Bool("pass_credentials_all", passCredentials),
	)

	pull := func() error {
		if repoURL != "" {
			chartURL, err := chartrepo.FindChartInRepoURL(repoURL, chartRef, getters,
				chartrepo.WithChartVersion(version),
				chartrepo.WithUsernamePassword(username, password),
				chartrepo.WithClientTLS(certFile, keyFile, caFile),
				chartrepo.WithInsecureSkipTLSVerify(insecureSkipVerify),
				chartrepo.WithPassCredentialsAll(passCredentials),
			)
			if err != nil {
				return fmt.Errorf("find chart in repo: %w", err)
			}

			chartRef = chartURL
		}

		saved, _, err := dl.DownloadTo(chartRef, version, tempDest)
		if err != nil {
			return fmt.Errorf("download chart: %w", err)
		}

		logger.DebugContext(ctx, "moving pulled chart",
			slog.String("src", saved),
			slog.String("dst", dstPath),
		)

		err = os.Rename(saved, dstPath)
		if err != nil {
			return fmt.Errorf("rename file from %q to %q: %w", saved, dstPath, err)
		}

		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- pull()
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

	return nil
}
