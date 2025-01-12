package helm

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/resource"

	argohelm "github.com/MacroPower/kclipper/pkg/argoutil/helm"
	"github.com/MacroPower/kclipper/pkg/helmrepo"
)

var DefaultClient = MustNewClient(
	NewTempPaths(os.TempDir(), NewBase64PathEncoder()),
	os.Getenv("ARGOCD_APP_PROJECT_NAME"),
	"10M",
)

type PathCacher interface {
	Add(key string, value string)
	GetPath(key string) (string, error)
	GetPathIfExists(key string) string
	GetPaths() map[string]string
}

type Client struct {
	Paths          PathCacher
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
func (c *Client) Pull(chart, repoURL, targetRevision string, creds helmrepo.Creds) (string, error) {
	p, _, err := c.PullWithCreds(chart, repoURL, targetRevision, creds, false)
	return p, err
}

// PullAndExtract will retrieve the chart, extract it, and return the path to the
// extracted chart. An [io.Closer] is also returned, calling Close() will clean up
// the extracted chart. Pulled charts will be stored in the injected [PathCacher]
// in .tar.gz format, and subsequent requests will try to use [PathCacher] rather
// than re-pulling the chart.
func (c *Client) PullAndExtract(
	chart, repoURL, targetRevision string, creds helmrepo.Creds,
) (string, io.Closer, error) {
	return c.PullWithCreds(chart, repoURL, targetRevision, creds, true)
}

func (c *Client) PullWithCreds(
	chart, repoURL, targetRevision string, creds helmrepo.Creds, extract bool,
) (string, io.Closer, error) {
	repoNetURL, err := url.Parse(repoURL)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse repoURL '%s': %w", repoURL, err)
	}

	isLocal := repoNetURL.Hostname() == ""
	if isLocal {
		chartDir, err := filepath.Abs(repoURL)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get absolute path: %w", err)
		}
		chartPath := filepath.Join(chartDir, chart)
		if !dirExists(chartPath) {
			return "", nil, fmt.Errorf("chart directory does not exist: %s", chartPath)
		}
		return chartPath, io.NopCloser(bytes.NewReader(nil)), nil
	}

	enableOCI := repoNetURL.Scheme == ""

	argoCreds := argohelm.Creds{
		Username:           creds.Username,
		Password:           creds.Password,
		CAPath:             creds.CAPath,
		CertData:           creds.CertData,
		KeyData:            creds.KeyData,
		InsecureSkipVerify: creds.InsecureSkipVerify,
	}

	ahc := argohelm.NewClient(repoNetURL.String(), argoCreds, enableOCI, c.Proxy, c.NoProxy,
		argohelm.WithChartPaths(c.Paths))

	var chartPath string
	if !extract {
		chartPath, err = ahc.PullChart(chart, targetRevision, c.Project, creds.PassCredentials,
			c.MaxExtractSize.Value(), c.MaxExtractSize.IsZero())
		if err != nil {
			return "", nil, fmt.Errorf("error extracting helm chart: %w", err)
		}
		return chartPath, nil, nil
	}

	chartPath, closer, err := ahc.ExtractChart(chart, targetRevision, c.Project, creds.PassCredentials,
		c.MaxExtractSize.Value(), c.MaxExtractSize.IsZero())
	if err != nil {
		return "", closer, fmt.Errorf("error extracting helm chart: %w", err)
	}
	return chartPath, closer, nil
}

func dirExists(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil || !fi.IsDir() {
		return false
	}
	return true
}
