package helmtest

import (
	"errors"
	"fmt"
	"go/build"
	"io"
	"path/filepath"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/helmrepo"
)

var (
	DefaultTestClient ChartClient
	ErrTestClient     = errors.New("test client failure")
)

func init() {
	pkg, err := build.Default.Import("github.com/MacroPower/kclipper/pkg/helmtest", ".", build.FindOnly)
	if err != nil {
		panic(fmt.Errorf("failed to find package: %w", err))
	}

	testDataDir := filepath.Join(pkg.Dir, "testdata")
	DefaultTestClient = &TestClient{
		BaseClient: helm.MustNewClient(
			helm.NewTempPaths(testDataDir, &TestPathEncoder{}), helmrepo.DefaultManager, "test", "10M",
		),
	}
}

type ChartClient interface {
	Pull(chart, repoURL, targetRevision string) (string, error)
	PullAndExtract(chart, repoURL, targetRevision string) (string, io.Closer, error)
}

type TestClient struct {
	BaseClient ChartClient
}

func (c *TestClient) Pull(chart, repoURL, targetRevision string) (string, error) {
	p, _, err := c.pull(chart, repoURL, targetRevision, false)
	return p, err
}

func (c *TestClient) PullAndExtract(chart, repoURL, targetRevision string) (string, io.Closer, error) {
	return c.pull(chart, repoURL, targetRevision, true)
}

func (c *TestClient) pull(chart, repoURL, targetRevision string, extract bool) (string, io.Closer, error) {
	if extract {
		chartPath, closer, err := c.BaseClient.PullAndExtract(chart, repoURL, targetRevision)
		if err != nil {
			return "", nil, fmt.Errorf("%w: %w", ErrTestClient, err)
		}
		return chartPath, closer, nil
	}
	chartPath, err := c.BaseClient.Pull(chart, repoURL, targetRevision)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrTestClient, err)
	}
	return chartPath, nil, nil
}
