package chartcmd_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/chartcmd"
	"github.com/macropower/kclipper/pkg/helmtest"
	"github.com/macropower/kclipper/pkg/kclmodule/kclchart"
)

const (
	updateBasePath = "testdata/update"
)

func TestHelmChartUpdate(t *testing.T) {
	t.Parallel()

	chartPath := path.Join(updateBasePath, "charts")
	os.RemoveAll(path.Join(chartPath, "podinfo"))

	chartPkg, err := chartcmd.NewKCLPackage(chartPath, helmtest.DefaultTestClient)
	require.NoError(t, err)

	_, err = chartPkg.Init()
	require.NoError(t, err)

	err = chartPkg.AddChart("podinfo", &kclchart.ChartConfig{
		ChartBase: kclchart.ChartBase{
			Chart:          "podinfo",
			RepoURL:        "https://stefanprodan.github.io/podinfo",
			TargetRevision: "6.7.1",
		},
	})
	require.NoError(t, err)
	os.RemoveAll(path.Join(chartPath, "podinfo"))

	err = chartPkg.Update()
	require.NoError(t, err)
}
