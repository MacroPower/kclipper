package helm

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/pathutil"
	"github.com/MacroPower/kclipper/pkg/syncutil"
)

var globalLock = syncutil.NewKeyLock()

var DefaultClient = MustNewClient(
	pathutil.NewStaticTempPaths(filepath.Join(os.TempDir(), "charts"), pathutil.NewBase64PathEncoder()),
	os.Getenv("ARGOCD_APP_PROJECT_NAME"),
	"10M",
)

type PathCacher interface {
	Add(key string, value string)
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
	Project        string
	Proxy          string
	NoProxy        string
}

func NewClient(paths PathCacher, project, maxExtractSize string) (*Client, error) {
	maxExtractSizeResource, err := resource.ParseQuantity(maxExtractSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse quantity '%s': %w", maxExtractSize, err)
	}

	return &Client{
		Paths:          paths,
		RepoLock:       globalLock,
		MaxExtractSize: maxExtractSizeResource,
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

// Pull will retrieve the chart, and return the path to the chart .tar.gz file.
// Pulled charts will be stored in the injected [PathCacher], and subsequent
// requests will try to use [PathCacher] rather than re-pulling the chart.
func (c *Client) Pull(chart, repo, version string, repos helmrepo.Getter) (string, error) {
	p, _, err := c.pull(chart, repo, version, false, repos)
	return p, err
}

// PullAndExtract will retrieve the chart, extract it, and return the path to the
// extracted chart. An [io.Closer] is also returned, calling Close() will clean up
// the extracted chart. Pulled charts will be stored in the injected [PathCacher]
// in .tar.gz format, and subsequent requests will try to use [PathCacher] rather
// than re-pulling the chart.
func (c *Client) PullAndExtract(chart, repo, version string, repos helmrepo.Getter) (string, io.Closer, error) {
	return c.pull(chart, repo, version, true, repos)
}

func (c *Client) pull(chart, repo, version string, extract bool, repos helmrepo.Getter) (string, io.Closer, error) {
	hr, err := repos.Get(repo)
	if err != nil {
		return "", nil, fmt.Errorf("error getting repo %s: %w", repo, err)
	}

	if hr.IsLocal() {
		chartPath, err := c.getLocalChart(chart, hr)
		if err != nil {
			return "", nil, fmt.Errorf("error getting local chart: %w", err)
		}
		return chartPath, nil, err
	}

	chartPath, err := c.getCachedOrRemoteChart(chart, version, hr)
	if err != nil {
		return "", nil, fmt.Errorf("error pulling helm chart: %w", err)
	}

	if !extract {
		return chartPath, nil, nil
	}

	var closer io.Closer
	chartPath, closer, err = c.extractChart(chart, chartPath)
	if err != nil {
		return "", nil, fmt.Errorf("error extracting helm chart: %w", err)
	}

	return chartPath, closer, nil
}

func (c *Client) getLocalChart(chart string, repo *helmrepo.Repo) (string, error) {
	chartPath := filepath.Join(repo.URL.String(), chart)
	if !dirExists(chartPath) {
		return "", fmt.Errorf("chart directory does not exist: %s", chartPath)
	}
	return chartPath, nil
}

func (c *Client) getCachedOrRemoteChart(chart, version string, repo *helmrepo.Repo) (string, error) {
	cachedChartPath, err := c.getCachedChartPath(chart, repo.URL.String(), version)
	if err != nil {
		return "", fmt.Errorf("error getting cached chart path: %w", err)
	}

	c.RepoLock.Lock(cachedChartPath)
	defer c.RepoLock.Unlock(cachedChartPath)

	// check if chart tar is already downloaded
	exists, err := fileExists(cachedChartPath)
	if err != nil {
		return "", fmt.Errorf("error checking existence of cached chart path: %w", err)
	}

	if !exists {
		err := c.pullRemoteChart(chart, version, cachedChartPath, repo)
		if err != nil {
			return "", fmt.Errorf("error pulling helm chart: %w", err)
		}
	}

	return cachedChartPath, nil
}

func (c *Client) pullRemoteChart(chart, version, dstPath string, repo *helmrepo.Repo) error {
	helmCmd, err := NewCmdWithVersion("", c.Proxy, c.NoProxy)
	if err != nil {
		return fmt.Errorf("error creating Helm command: %w", err)
	}

	// create empty temp directory to extract chart from the registry
	tempDest, err := createTempDir(os.TempDir())
	if err != nil {
		return fmt.Errorf("error creating temporary destination directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDest) }()

	_, err = helmCmd.Fetch(chart, version, tempDest, repo)
	if err != nil {
		return fmt.Errorf("error fetching chart: %w", err)
	}

	// 'helm pull/fetch' file downloads chart into the tgz file and we move that to where we want it
	infos, err := os.ReadDir(tempDest)
	if err != nil {
		return fmt.Errorf("error reading directory %s: %w", tempDest, err)
	}
	if len(infos) != 1 {
		return fmt.Errorf("expected 1 file, found %v", len(infos))
	}

	chartFilePath := filepath.Join(tempDest, infos[0].Name())
	err = os.Rename(chartFilePath, dstPath)
	if err != nil {
		return fmt.Errorf("error renaming file from %s to %s: %w", chartFilePath, dstPath, err)
	}
	return nil
}

func (c *Client) extractChart(chart, srcPath string) (string, io.Closer, error) {
	tempDest, err := createTempDir(os.TempDir())
	if err != nil {
		return "", nil, fmt.Errorf("error creating temporary destination directory: %w", err)
	}

	reader, err := os.Open(srcPath)
	if err != nil {
		return "", nil, fmt.Errorf("error opening chart path %s: %w", srcPath, err)
	}
	err = gunzip(tempDest, reader, c.MaxExtractSize.Value(), false)
	if err != nil {
		_ = os.RemoveAll(tempDest)
		return "", nil, fmt.Errorf("error untarring chart: %w", err)
	}

	return filepath.Join(tempDest, normalizeChartName(chart)), newInlineCloser(func() error {
		return os.RemoveAll(tempDest)
	}), nil
}

func (c *Client) CleanChartCache(chart, repo, version string) error {
	cachePath, err := c.getCachedChartPath(chart, repo, version)
	if err != nil {
		return fmt.Errorf("error getting cached chart path: %w", err)
	}
	if err := os.RemoveAll(cachePath); err != nil {
		return fmt.Errorf("error removing chart cache at %s: %w", cachePath, err)
	}
	return nil
}

func (c *Client) getCachedChartPath(chart, repo, version string) (string, error) {
	keyData, err := json.Marshal(map[string]string{"url": repo, "chart": chart, "version": version, "project": c.Project})
	if err != nil {
		return "", fmt.Errorf("error marshaling cache key data: %w", err)
	}
	chartPath, err := c.Paths.GetPath(string(keyData))
	if err != nil {
		return "", fmt.Errorf("error getting chart cache path: %w", err)
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
