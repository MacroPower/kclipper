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
			helm.NewTempPaths(testDataDir, &TestPathEncoder{}), "test", "10M",
		),
	}
}

type ChartClient interface {
	Pull(chart, repoURL, targetRevision string, repos helmrepo.Getter) (string, error)
	PullAndExtract(chart, repoURL, targetRevision string, repos helmrepo.Getter) (string, io.Closer, error)
}

type TestClient struct {
	BaseClient ChartClient
}

func (c *TestClient) Pull(chart, repo, version string, repos helmrepo.Getter) (string, error) {
	p, _, err := c.pull(chart, repo, version, false, repos)
	return p, err
}

func (c *TestClient) PullAndExtract(chart, repo, version string, repos helmrepo.Getter) (string, io.Closer, error) {
	return c.pull(chart, repo, version, true, repos)
}

func (c *TestClient) pull(chart, repo, version string, extract bool, repos helmrepo.Getter) (string, io.Closer, error) {
	if extract {
		chartPath, closer, err := c.BaseClient.PullAndExtract(chart, repo, version, repos)
		if err != nil {
			return "", nil, fmt.Errorf("%w: %w", ErrTestClient, err)
		}
		return chartPath, closer, nil
	}
	chartPath, err := c.BaseClient.Pull(chart, repo, version, repos)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrTestClient, err)
	}
	return chartPath, nil, nil
}
