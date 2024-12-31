package helm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/MacroPower/kclx/pkg/helm"
	"github.com/MacroPower/kclx/pkg/jsonschema"
)

func TestHelmChart(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		opts  helm.TemplateOpts
		gen   jsonschema.FileGenerator
		match func(string) bool
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

			c := helm.NewChart(helm.DefaultClient, tc.opts)

			results, err := c.Template()
			require.NoError(t, err)
			require.NotEmpty(t, results)
			resultYAMLs, err := yaml.Marshal(results)
			require.NoError(t, err)
			require.NotEmpty(t, resultYAMLs)

			schema, err := c.GetValuesJSONSchema(tc.gen, tc.match)
			require.NoError(t, err)
			require.NotEmpty(t, schema)

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
