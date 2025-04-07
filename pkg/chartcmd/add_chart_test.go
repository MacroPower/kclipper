package chartcmd_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"kcl-lang.io/cli/pkg/options"
	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/chartcmd"
	"github.com/MacroPower/kclipper/pkg/crd"
	"github.com/MacroPower/kclipper/pkg/helmtest"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/kclmodule/kclchart"
	"github.com/MacroPower/kclipper/pkg/kclmodule/kclhelm"
)

const (
	addBasePath = "testdata/add"
)

func TestHelmChartAdd(t *testing.T) {
	t.Parallel()

	chartPath := path.Join(addBasePath, "charts")
	os.RemoveAll(chartPath)

	ca, err := chartcmd.NewKCLPackage(chartPath, helmtest.DefaultTestClient)
	require.NoError(t, err)

	_, err = ca.Init()
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
					SkipHooks:       true,
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: jsonschema.AutoGeneratorType,
				},
			},
		},
		"app_template": {
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
		"prometheus_crd_file": {
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
					CRDGenerator:    crd.GeneratorTypeChartPath,
					CRDPaths:        []string{"**/crds/crds/*.yaml"},
				},
			},
		},
		"prometheus_crd_template": {
			chart: &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:          "kube-prometheus-stack",
					RepoURL:        "https://prometheus-community.github.io/helm-charts",
					TargetRevision: "67.9.0",
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: jsonschema.AutoGeneratorType,
					CRDGenerator:    crd.GeneratorTypeTemplate,
				},
			},
		},
		"simple_chart_relative": {
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
		"simple_chart_absolute": {
			chart: &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:   "simple-chart",
					RepoURL: "/pkg/chartcmd/testdata/charts",
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: jsonschema.LocalPathGeneratorType,
					SchemaPath:      "/pkg/chartcmd/testdata/schemas/simple-chart/values.schema.json",
				},
			},
		},
		"simple_chart_no_config": {
			chart: &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:   "simple-chart",
					RepoURL: "./charts",
				},
			},
		},
		"grafana_operator_oci": {
			chart: &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:          "grafana-operator",
					RepoURL:        "oci://ghcr.io/grafana/helm-charts/grafana-operator",
					TargetRevision: "v5.16.0",
					Values: map[string]any{
						"rbac": map[string]any{
							"create": false,
						},
					},
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: jsonschema.AutoGeneratorType,
					CRDGenerator:    crd.GeneratorTypeTemplate,
				},
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := ca.AddChart(name, tc.chart)
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
