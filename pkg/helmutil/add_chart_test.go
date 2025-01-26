package helmutil_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"kcl-lang.io/cli/pkg/options"
	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/helmtest"
	"github.com/MacroPower/kclipper/pkg/helmutil"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/kclchart"
	"github.com/MacroPower/kclipper/pkg/kclhelm"
)

const (
	addBasePath = "testdata/add"
)

func TestHelmChartAdd(t *testing.T) {
	t.Parallel()

	chartPath := path.Join(addBasePath, "charts")
	os.RemoveAll(chartPath)

	ca := helmutil.NewChartPkg(chartPath, helmtest.DefaultTestClient)

	err := ca.Init()
	require.NoError(t, err)

	tcs := map[string]struct {
		chart *kclchart.ChartConfig
	}{
		"podinfo": {
			chart: &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:           "podinfo",
					RepoURL:         "https://stefanprodan.github.io/podinfo",
					TargetRevision:  "6.7.1",
					SchemaValidator: jsonschema.HelmValidatorType,
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: jsonschema.AutoGeneratorType,
				},
			},
		},
		"app-template": {
			chart: &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:          "app-template",
					RepoURL:        "https://bjw-s.github.io/helm-charts/",
					TargetRevision: "3.6.0",
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: jsonschema.ChartPathGeneratorType,
					SchemaPath:      "charts/common/values.schema.json",
				},
			},
		},
		"prometheus": {
			chart: &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:          "kube-prometheus-stack",
					RepoURL:        "@prometheus",
					TargetRevision: "67.9.0",
					Repositories: []kclhelm.ChartRepo{
						{
							Name: "prometheus",
							URL:  "https://prometheus-community.github.io/helm-charts",
						},
					},
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: jsonschema.AutoGeneratorType,
					CRDPath:         "**/crds/crds/*.yaml",
				},
			},
		},
		"simple-chart-rel": {
			chart: &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:   "simple-chart",
					RepoURL: "./charts",
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: jsonschema.LocalPathGeneratorType,
					SchemaPath:      "./schemas/simple-chart/values.schema.json",
				},
			},
		},
		"simple-chart-abs": {
			chart: &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:   "simple-chart",
					RepoURL: "/pkg/helmutil/testdata/charts",
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: jsonschema.LocalPathGeneratorType,
					SchemaPath:      "/pkg/helmutil/testdata/schemas/simple-chart/values.schema.json",
				},
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := ca.AddChart(tc.chart)
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
