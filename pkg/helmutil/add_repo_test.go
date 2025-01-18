package helmutil_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"kcl-lang.io/cli/pkg/options"
	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/helmmodels/pluginmodule"
	"github.com/MacroPower/kclipper/pkg/helmtest"
	"github.com/MacroPower/kclipper/pkg/helmutil"
)

const (
	addRepoBasePath = "testdata/add-repo"
)

func TestHelmChartAddRepo(t *testing.T) {
	t.Parallel()

	chartPath := path.Join(addRepoBasePath, "charts")
	os.RemoveAll(chartPath)

	ca := helmutil.NewChartPkg(chartPath, helmtest.DefaultTestClient)

	err := ca.Init()
	require.NoError(t, err)

	tcs := map[string]struct {
		repo *pluginmodule.ChartRepo
	}{
		"podinfo": {
			repo: &pluginmodule.ChartRepo{
				Name:            "podinfo",
				URL:             "https://stefanprodan.github.io/podinfo",
				PassCredentials: true,
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := ca.AddRepo(tc.repo)
			require.NoError(t, err)

			depsOpt, err := options.LoadDepsFrom(chartPath, true)
			require.NoError(t, err)
			results, err := kcl.Test(
				&kcl.TestOptions{
					PkgList:  []string{chartPath},
					FailFast: true,
				},
				*depsOpt,
			)
			require.NoError(t, err)

			require.Emptyf(t, results.Info, "expected no errors, got %d: %#v", len(results.Info), results)
		})
	}
}
