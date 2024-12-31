package helm

import (
	"fmt"
	"io"
	"net/url"
	"os"

	argohelm "github.com/argoproj/argo-cd/v2/util/helm"
	"k8s.io/apimachinery/pkg/api/resource"
)

var DefaultClient = MustNewClient(NewTempPaths(os.TempDir()), os.Getenv("ARGOCD_APP_PROJECT_NAME"), "10M")

type TempPaths interface {
	Add(key string, value string)
	GetPath(key string) (string, error)
	GetPathIfExists(key string) string
	GetPaths() map[string]string
}

type Creds struct {
	Username           string
	Password           string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
}

type Client struct {
	paths          TempPaths
	maxExtractSize resource.Quantity
	project        string
}

func NewClient(paths TempPaths, project, maxExtractSize string) (*Client, error) {
	maxExtractSizeResource, err := resource.ParseQuantity(maxExtractSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse quantity '%s': %w", maxExtractSize, err)
	}

	return &Client{
		paths:          paths,
		maxExtractSize: maxExtractSizeResource,
		project:        project,
	}, nil
}

// MustNewClient runs [NewClient] and panics on any errors.
func MustNewClient(paths TempPaths, project, maxExtractSize string) *Client {
	c, err := NewClient(paths, project, maxExtractSize)
	if err != nil {
		panic(err)
	}
	return c
}

// Pull will retrieve the chart, extract it, and return the path to the
// extracted chart. An io.Closer is also returned, calling Close() will clean up
// the extracted chart. Pulled charts will be stored in the injected [TempPaths]
// in .tar.gz format, and subsequent requests will try to use [TempPaths] rather
// than re-pulling the chart.
func (c *Client) Pull(chart, repoURL, targetRevision string) (string, io.Closer, error) {
	return c.PullWithCreds(chart, repoURL, targetRevision, Creds{}, false)
}

func (c *Client) PullWithCreds(
	chart, repoURL, targetRevision string, creds Creds, passCredentials bool,
) (string, io.Closer, error) {
	repoNetURL, err := url.Parse(repoURL)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse repoURL '%s': %w", repoURL, err)
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

	hcl := argohelm.NewClient(repoNetURL.String(), argoCreds, enableOCI, "", "",
		argohelm.WithChartPaths(c.paths))

	chartPath, closer, err := hcl.ExtractChart(chart, targetRevision, c.project, passCredentials,
		c.maxExtractSize.Value(), c.maxExtractSize.IsZero())
	if err != nil {
		return "", closer, fmt.Errorf("error extracting helm chart: %w", err)
	}

	return chartPath, closer, nil
}
