package helmtest

import (
	"fmt"
	"go/build"
	"io"
	"path/filepath"

	"github.com/MacroPower/kclx/pkg/helm"
)

var DefaultTestClient helm.ChartClient

func init() {
	pkg, _ := build.Default.Import("github.com/MacroPower/kclx/pkg/helmtest", ".", build.FindOnly)
	testDataDir := filepath.Join(pkg.Dir, "testdata")

	DefaultTestClient = &TestClient{
		BaseClient: helm.MustNewClient(helm.NewTempPaths(testDataDir, &TestPathEncoder{}), "test", "10M"),
	}
}

type ChartClient interface {
	Pull(chart, repoURL, targetRevision string) (string, io.Closer, error)
}

type TestClient struct {
	BaseClient ChartClient
}

func (c *TestClient) Pull(chart, repoURL, targetRevision string) (string, io.Closer, error) {
	return c.PullWithCreds(chart, repoURL, targetRevision, helm.Creds{}, false)
}

func (c *TestClient) PullWithCreds(
	chart, repoURL, targetRevision string, _ helm.Creds, _ bool,
) (string, io.Closer, error) {
	chartPath, closer, err := c.BaseClient.Pull(chart, repoURL, targetRevision)
	if err != nil {
		return "", closer, fmt.Errorf("error pulling helm chart: %w", err)
	}
	return chartPath, closer, nil
}
