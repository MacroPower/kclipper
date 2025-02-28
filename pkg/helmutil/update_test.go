package helmutil_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/helmtest"
	"github.com/MacroPower/kclipper/pkg/helmutil"
	"github.com/MacroPower/kclipper/pkg/kclchart"
)

const (
	updateBasePath = "testdata/update"
)

func TestHelmChartUpdate(t *testing.T) {
	t.Parallel()

	chartPath := path.Join(updateBasePath, "charts")
	os.RemoveAll(path.Join(chartPath, "podinfo"))

	chartPkg := helmutil.NewChartPkg(chartPath, helmtest.DefaultTestClient)

	_, err := chartPkg.Init()
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
