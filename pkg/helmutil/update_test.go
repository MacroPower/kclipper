package helmutil_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclx/pkg/helmutil"
	"github.com/MacroPower/kclx/pkg/jsonschema"
)

const (
	updateBasePath = "testdata/update"
)

func TestHelmChartUpdate(t *testing.T) {
	t.Parallel()

	chartPath := path.Join(updateBasePath, "charts")
	os.RemoveAll(path.Join(chartPath, "podinfo"))

	chartPkg := helmutil.NewChartPkg(chartPath)

	err := chartPkg.Init()
	require.NoError(t, err)

	err = chartPkg.Add("podinfo", "https://stefanprodan.github.io/podinfo", "6.7.1", "", jsonschema.AutoGeneratorType)
	require.NoError(t, err)
	os.RemoveAll(path.Join(chartPath, "podinfo"))

	err = chartPkg.Update()
	require.NoError(t, err)
}
