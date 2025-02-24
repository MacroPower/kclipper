package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/pathutil"
	"github.com/MacroPower/kclipper/pkg/syncutil"
)

var (
	globalLock = syncutil.NewKeyLock()

	DefaultClient = MustNewClient(
		pathutil.NewStaticTempPaths(filepath.Join(os.TempDir(), "charts"), pathutil.NewBase64PathEncoder()),
		os.Getenv("ARGOCD_APP_PROJECT_NAME"),
		"10M",
	)
)

type PathCacher interface {
	Add(key, value string)
	GetPath(key string) (string, error)
	GetPathIfExists(key string) string
	GetPaths() map[string]string
}

type KeyLocker interface {
	Lock(key string)
	Unlock(key string)
	RLock(key string)
	RUnlock(key string)
}

type Client struct {
	Paths          PathCacher
	RepoLock       KeyLocker
	MaxExtractSize resource.Quantity
	rc             *registry.Client
	helmHome       string
	Project        string
	Proxy          string
	NoProxy        string
}

func NewClient(paths PathCacher, project, maxExtractSize string) (*Client, error) {
	maxExtractSizeResource, err := resource.ParseQuantity(maxExtractSize)
	if err != nil {
		return nil, fmt.Errorf("parse quantity %q: %w", maxExtractSize, err)
	}

	rc, err := registry.NewClient(registry.ClientOptEnableCache(true))
	if err != nil {
		return nil, fmt.Errorf("create registry client: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return nil, fmt.Errorf("create temporary directory for helm: %w", err)
	}

	return &Client{
		Paths:          paths,
		RepoLock:       globalLock,
		MaxExtractSize: maxExtractSizeResource,
		rc:             rc,
		helmHome:       tmpDir,
		Project:        project,
	}, nil
}

// MustNewClient runs [NewClient] and panics on any errors.
func MustNewClient(paths PathCacher, project, maxExtractSize string) *Client {
	c, err := NewClient(paths, project, maxExtractSize)
	if err != nil {
		panic(err)
	}

	return c
}

// Pull pulls the Helm chart and returns the path to the chart directory or
// .tar.gz file. Pulled charts will be stored in the injected [PathCacher], and
// subsequent requests will try to use [PathCacher] rather than re-pulling the
// chart.
func (c *Client) Pull(ctx context.Context, chart, repo, version string, repos helmrepo.Getter) (string, error) {
	hr, err := repos.Get(repo)
	if err != nil {
		return "", fmt.Errorf("get repo: %q: %w", repo, err)
	}

	if hr.IsLocal() {
		chartPath, err := c.getLocalChart(chart, hr)
		if err != nil {
			return "", fmt.Errorf("get local chart: %w", err)
		}

		return chartPath, err
	}

	chartPath, err := c.getCachedOrRemoteChart(ctx, chart, version, hr)
	if err != nil {
		return "", fmt.Errorf("get cached or remote chart: %w", err)
	}

	return chartPath, err
}

// PullAndExtract will retrieve the chart, extract it (if it is a .tar.gz file),
// and return the path to the extracted chart. An [io.Closer] is also returned,
// calling Close() will clean up the extracted chart. Pulled charts will be
// stored in the injected [PathCacher] in .tar.gz format, and subsequent
// requests will try to use [PathCacher] rather than re-pulling the chart.
func (c *Client) PullAndExtract(ctx context.Context, chart, repo, version string, repos helmrepo.Getter) (string, io.Closer, error) {
	closer := NewNopCloser()

	chartPath, err := c.Pull(ctx, chart, repo, version, repos)
	if err != nil {
		return "", nil, fmt.Errorf("pull chart: %w", err)
	}

	// If the chart is already extracted, return the path to the extracted chart.
	if dirExists(chartPath) {
		return chartPath, closer, nil
	}

	chartPath, closer, err = c.extractChart(chart, chartPath)
	if err != nil {
		return "", nil, fmt.Errorf("extract chart: %w", err)
	}

	return chartPath, closer, nil
}

func (c *Client) getLocalChart(chart string, repo *helmrepo.Repo) (string, error) {
	chartPath := filepath.Join(repo.URL.String(), chart)
	if !dirExists(chartPath) {
		return "", fmt.Errorf("chart directory does not exist: %q", chartPath)
	}

	return chartPath, nil
}

func (c *Client) getCachedOrRemoteChart(ctx context.Context, chart, version string, repo *helmrepo.Repo) (string, error) {
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
	tempDest, err := createTempDir(os.TempDir())
	if err != nil {
		return fmt.Errorf("create temporary destination directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDest) }()

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
		ap.RepoURL = repo.URL.String()
		ap.Username = repo.Username
		ap.Password = repo.Password
		ap.CaFile = repo.CAPath.String()
		ap.CertFile = repo.TLSClientCertDataPath.String()
		ap.KeyFile = repo.TLSClientCertKeyPath.String()
		ap.PassCredentialsAll = repo.PassCredentials
		ap.InsecureSkipTLSverify = repo.InsecureSkipVerify
	}

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
	err = os.Rename(chartFilePath, dstPath)
	if err != nil {
		return fmt.Errorf("rename file from %q to %q: %w", chartFilePath, dstPath, err)
	}

	return nil
}

func (c *Client) extractChart(chart, srcPath string) (string, io.Closer, error) {
	tempDest, err := createTempDir(os.TempDir())
	if err != nil {
		return "", nil, fmt.Errorf("create temporary directory: %w", err)
	}

	//nolint:gosec // G304 checked by repo resolver.
	reader, err := os.Open(srcPath)
	if err != nil {
		return "", nil, fmt.Errorf("open chart path %q: %w", srcPath, err)
	}

	err = gunzip(tempDest, reader, c.MaxExtractSize.Value(), false)
	if err != nil {
		_ = os.RemoveAll(tempDest)

		return "", nil, fmt.Errorf("gunzip chart: %w", err)
	}

	return filepath.Join(tempDest, normalizeChartName(chart)), newInlineCloser(func() error {
		return os.RemoveAll(tempDest)
	}), nil
}

func (c *Client) CleanChartCache(chart, repo, version string) error {
	cachePath, err := c.getCachedChartPath(chart, repo, version)
	if err != nil {
		return fmt.Errorf("get cached chart path: %w", err)
	}

	if err := os.RemoveAll(cachePath); err != nil {
		return fmt.Errorf("remove chart cache at %q: %w", cachePath, err)
	}

	return nil
}

func (c *Client) getCachedChartPath(chart, repo, version string) (string, error) {
	keyData, err := json.Marshal(map[string]string{"url": repo, "chart": chart, "version": version, "project": c.Project})
	if err != nil {
		return "", fmt.Errorf("marshal key data: %w", err)
	}

	chartPath, err := c.Paths.GetPath(string(keyData))
	if err != nil {
		return "", fmt.Errorf("get path: %w", err)
	}

	return chartPath, nil
}

// Normalize a chart name for file system use, that is, if chart name is
// foo/bar/baz, returns the last component as chart name.
func normalizeChartName(chart string) string {
	strings.Join(strings.Split(chart, "/"), "_")
	_, nc := path.Split(chart)
	// We do not want to return the empty string or something else related to
	// filesystem access. Instead, return original string.
	if nc == "" || nc == "." || nc == ".." {
		return chart
	}

	return nc
}
