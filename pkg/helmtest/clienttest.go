package helmtest

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"io"
	"path/filepath"
	"time"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/pathutil"
)

var (
	DefaultTestClient ChartClient
	SlowTestClient    ChartClient

	ErrTestClient = errors.New("test client failure")
)

func init() {
	pkg, err := build.Default.Import("github.com/MacroPower/kclipper/pkg/helmtest", ".", build.FindOnly)
	if err != nil {
		panic(fmt.Errorf("failed to find package: %w", err))
	}

	testDataDir := filepath.Join(pkg.Dir, "testdata")
	DefaultTestClient = &TestClient{
		BaseClient: helm.MustNewClient(
			pathutil.NewStaticTempPaths(filepath.Join(testDataDir, "charts"), &TestPathEncoder{}),
			"test",
			"10M",
		),
	}
	SlowTestClient = &TestClient{
		BaseClient: helm.MustNewClient(
			pathutil.NewStaticTempPaths(filepath.Join(testDataDir, "charts"), &TestPathEncoder{}),
			"test",
			"10M",
		),
		Latency: 1 * time.Second,
	}
}

type ChartClient interface {
	Pull(ctx context.Context, chart, repoURL, targetRevision string, repos helmrepo.Getter) (string, error)
	PullAndExtract(ctx context.Context, chart, repoURL, targetRevision string, repos helmrepo.Getter) (string, io.Closer, error)
}

type TestClient struct {
	BaseClient ChartClient
	Latency    time.Duration
}

func (c *TestClient) Pull(ctx context.Context, chart, repo, version string, repos helmrepo.Getter) (string, error) {
	p, _, err := c.pull(ctx, chart, repo, version, false, repos)

	return p, err
}

func (c *TestClient) PullAndExtract(ctx context.Context, chart, repo, version string, repos helmrepo.Getter) (string, io.Closer, error) {
	return c.pull(ctx, chart, repo, version, true, repos)
}

//nolint:revive // TODO: Refactor this.
func (c *TestClient) pull(ctx context.Context, chart, repo, version string, extract bool, repos helmrepo.Getter) (string, io.Closer, error) {
	time.Sleep(c.Latency)

	if extract {
		chartPath, closer, err := c.BaseClient.PullAndExtract(ctx, chart, repo, version, repos)
		if err != nil {
			return "", nil, fmt.Errorf("%w: %w", ErrTestClient, err)
		}

		return chartPath, closer, nil
	}

	chartPath, err := c.BaseClient.Pull(ctx, chart, repo, version, repos)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrTestClient, err)
	}

	return chartPath, nil, nil
}
