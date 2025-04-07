package helmtest

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"path/filepath"
	"time"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/paths"
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
			paths.NewStaticTempPaths(filepath.Join(testDataDir, "charts"), &TestPathEncoder{}),
			"test",
		),
	}
	SlowTestClient = &TestClient{
		BaseClient: helm.MustNewClient(
			paths.NewStaticTempPaths(filepath.Join(testDataDir, "charts"), &TestPathEncoder{}),
			"test",
		),
		Latency: 1 * time.Second,
	}
}

type ChartClient interface {
	Pull(ctx context.Context, chart, repoURL, targetRevision string, repos helmrepo.Getter) (*helm.PulledChart, error)
}

type TestClient struct {
	BaseClient ChartClient
	Latency    time.Duration
}

func (c *TestClient) Pull(ctx context.Context, chart, repo, version string, repos helmrepo.Getter) (*helm.PulledChart, error) {
	time.Sleep(c.Latency)

	pulledChart, err := c.BaseClient.Pull(ctx, chart, repo, version, repos)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTestClient, err)
	}

	return pulledChart, nil
}
