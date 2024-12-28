package helmutil_test

import (
	"os"
	"path"
	"testing"

	"github.com/MacroPower/kclx/pkg/helm/schemagen"
	"github.com/MacroPower/kclx/pkg/helmutil"
)

const (
	updateBasePath = "testdata/update"
)

func TestHelmChartUpdate(t *testing.T) {
	t.Parallel()

	// os.RemoveAll(updateBasePath)
	chartPath := path.Join(updateBasePath, "charts")

	chartPkg := helmutil.NewChartPkg(chartPath)

	if err := chartPkg.Init(); err != nil {
		t.Fatal(err)
	}

	err := chartPkg.Add("podinfo", "https://stefanprodan.github.io/podinfo", "6.7.1", "", "", schemagen.AutoGenerator)
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(path.Join(chartPath, "podinfo"))

	err = chartPkg.Update()
	if err != nil {
		t.Fatal(err)
	}
}
