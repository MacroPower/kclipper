package helm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/helmtest"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/log"
)

func init() {
	log.SetLogLevel("warn")
}

func TestHelmChart(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		opts  helm.TemplateOpts
		gen   jsonschema.FileGenerator
		match func(string) bool
		crds  bool
	}{
		"podinfo": {
			opts: helm.TemplateOpts{
				ChartName:      "podinfo",
				TargetRevision: "6.7.1",
				RepoURL:        "https://stefanprodan.github.io/podinfo",
			},
			gen:   jsonschema.DefaultAutoGenerator,
			match: jsonschema.GetFileFilter(jsonschema.AutoGeneratorType),
		},
		"prometheus": {
			opts: helm.TemplateOpts{
				ChartName:      "kube-prometheus-stack",
				TargetRevision: "67.9.0",
				RepoURL:        "https://prometheus-community.github.io/helm-charts",
			},
			gen:   jsonschema.DefaultAutoGenerator,
			match: jsonschema.GetFileFilter(jsonschema.AutoGeneratorType),
			crds:  true,
		},
		"app-template": {
			opts: helm.TemplateOpts{
				ChartName:      "app-template",
				TargetRevision: "3.6.0",
				RepoURL:        "https://bjw-s.github.io/helm-charts/",
				ValuesObject: map[string]any{
					"controllers": map[string]any{
						"main": map[string]any{
							"enabled": true,
							"containers": map[string]any{
								"main": map[string]any{
									"image": map[string]any{
										"repository": "nginx",
										"tag":        "latest",
									},
								},
							},
						},
					},
				},
			},
			gen:   jsonschema.DefaultReaderGenerator,
			match: func(s string) bool { return s == "charts/common/values.schema.json" },
		},
		"local": {
			opts: helm.TemplateOpts{
				ChartName: "simple-chart",
				RepoURL:   "./testdata",
			},
			gen:   jsonschema.DefaultAutoGenerator,
			match: jsonschema.GetFileFilter(jsonschema.AutoGeneratorType),
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, err := helm.NewChart(helmtest.DefaultTestClient, tc.opts)
			require.NoError(t, err)

			results, err := c.Template()
			require.NoError(t, err)
			require.NotEmpty(t, results)
			resultYAMLs, err := yaml.Marshal(results)
			require.NoError(t, err)
			require.NotEmpty(t, resultYAMLs)

			cf, err := helm.NewChartFiles(helmtest.DefaultTestClient, tc.opts)
			require.NoError(t, err)
			defer cf.Dispose()

			schema, err := cf.GetValuesJSONSchema(tc.gen, tc.match)
			require.NoError(t, err)
			require.NotEmpty(t, schema)

			if tc.crds {
				crds, err := cf.GetCRDs(func(s string) bool {
					return filepath.Base(filepath.Dir(s)) == "crds" && filepath.Base(s) != "Chart.yaml" && filepath.Ext(s) == ".yaml"
				})
				require.NoError(t, err)
				require.NotEmpty(t, crds)
				for _, crd := range crds {
					require.NotEmpty(t, crd)
				}
			}

			// Write the results to testdata/got for debugging.
			gotDir := filepath.Join("testdata/got", tc.opts.ChartName)
			err = os.RemoveAll(gotDir)
			require.NoError(t, err)
			err = os.MkdirAll(gotDir, 0o755)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(gotDir, "output.yaml"), resultYAMLs, 0o600)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(gotDir, "values.schema.json"), schema, 0o600)
			require.NoError(t, err)
		})
	}
}

func BenchmarkHelmChart(b *testing.B) {
	c, err := helm.NewChart(helmtest.DefaultTestClient, helm.TemplateOpts{
		ChartName:      "podinfo",
		TargetRevision: "6.7.1",
		RepoURL:        "https://stefanprodan.github.io/podinfo",
	})
	require.NoError(b, err)
	_, err = c.Template()
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := c.Template()
			require.NoError(b, err)
		}
	})
}

func BenchmarkAppTemplateHelmChart(b *testing.B) {
	c, err := helm.NewChart(helmtest.DefaultTestClient, helm.TemplateOpts{
		ChartName:      "app-template",
		TargetRevision: "3.6.0",
		RepoURL:        "https://bjw-s.github.io/helm-charts/",
	})
	require.NoError(b, err)
	_, err = c.Template()
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := c.Template()
			require.NoError(b, err)
		}
	})
}
