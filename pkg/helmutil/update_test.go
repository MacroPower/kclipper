package helmutil_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/helmtest"
	"github.com/MacroPower/kclipper/pkg/helmutil"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

const (
	updateBasePath = "testdata/update"
)

func TestHelmChartUpdate(t *testing.T) {
	t.Parallel()

	chartPath := path.Join(updateBasePath, "charts")
	os.RemoveAll(path.Join(chartPath, "podinfo"))

	chartPkg := helmutil.NewChartPkg(chartPath, helmtest.DefaultTestClient)

	err := chartPkg.Init()
	require.NoError(t, err)

	schemaPath := ""
	crdPath := ""
	err = chartPkg.Add("podinfo", "https://stefanprodan.github.io/podinfo", "6.7.1", schemaPath, crdPath,
		jsonschema.DefaultGeneratorType, jsonschema.DefaultValidatorType)
	require.NoError(t, err)
	os.RemoveAll(path.Join(chartPath, "podinfo"))

	err = chartPkg.Update()
	require.NoError(t, err)
}
