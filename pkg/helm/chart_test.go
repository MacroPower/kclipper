package helm_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/macropower/kclipper/pkg/crd"
	"github.com/macropower/kclipper/pkg/helm"
	"github.com/macropower/kclipper/pkg/helmrepo"
	"github.com/macropower/kclipper/pkg/helmtest"
	"github.com/macropower/kclipper/pkg/jsonschema"
)

func init() {
	err := helmrepo.DefaultManager.Add(&helmrepo.RepoOpts{
		Name:            "chartmuseum",
		URL:             "http://localhost:8080",
		Username:        "user",
		Password:        "hunter2",
		PassCredentials: true,
	})
	if err != nil {
		panic(err)
	}

	testdataDir := "./testdata"

	gotDir := filepath.Join(testdataDir, "got")
	err = os.RemoveAll(gotDir)
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(gotDir, 0o700)
	if err != nil {
		panic(err)
	}
}

func TestHelmChart(t *testing.T) {
	t.Parallel()

	maxSize := resource.NewQuantity(100000000, resource.BinarySI)

	tcs := map[string]struct {
		gen          jsonschema.FileGenerator
		match        func(string) bool
		opts         *helm.TemplateOpts
		objectCount  int
		importValues bool
		importCRDs   bool
	}{
		"podinfo": {
			opts: &helm.TemplateOpts{
				ChartName:      "podinfo",
				TargetRevision: "6.7.1",
				RepoURL:        "https://stefanprodan.github.io/podinfo",
			},
			gen:          jsonschema.DefaultAutoGenerator,
			match:        jsonschema.GetFileFilter(jsonschema.AutoGeneratorType),
			importValues: true,
			importCRDs:   false,
			objectCount:  -1,
		},
		"prometheus": {
			opts: &helm.TemplateOpts{
				ChartName:      "kube-prometheus-stack",
				TargetRevision: "67.9.0",
				RepoURL:        "https://prometheus-community.github.io/helm-charts",
			},
			gen:          jsonschema.DefaultAutoGenerator,
			match:        jsonschema.GetFileFilter(jsonschema.AutoGeneratorType),
			importValues: true,
			importCRDs:   true,
			objectCount:  -1,
		},
		"app-template": {
			opts: &helm.TemplateOpts{
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
				SkipSchemaValidation: true,
			},
			gen:          jsonschema.DefaultReaderGenerator,
			match:        func(s string) bool { return s == "charts/common/values.schema.json" },
			importValues: true,
			importCRDs:   false,
			objectCount:  -1,
		},
		"local path": {
			opts: &helm.TemplateOpts{
				ChartName: "simple-chart",
				RepoURL:   "./testdata",
			},
			gen:          jsonschema.DefaultAutoGenerator,
			match:        jsonschema.GetFileFilter(jsonschema.AutoGeneratorType),
			importValues: true,
			importCRDs:   false,
			objectCount:  4,
		},
		"direct repo with auth": {
			opts: &helm.TemplateOpts{
				ChartName:      "simple-chart",
				TargetRevision: "0.1.0",
				RepoURL:        "http://localhost:8080",
			},
			gen:          jsonschema.DefaultAutoGenerator,
			match:        jsonschema.GetFileFilter(jsonschema.AutoGeneratorType),
			importValues: true,
			importCRDs:   false,
			objectCount:  4,
		},
		"subchart uses repo with auth": {
			opts: &helm.TemplateOpts{
				ChartName: "parent-chart",
				RepoURL:   "./testdata",
			},
			gen:          jsonschema.DefaultAutoGenerator,
			match:        jsonschema.GetFileFilter(jsonschema.AutoGeneratorType),
			importValues: true,
			importCRDs:   false,
			objectCount:  4,
		},
		"crds only": {
			opts: &helm.TemplateOpts{
				ChartName: "crds",
				RepoURL:   "./testdata",
				SkipCRDs:  false,
			},
			gen:          jsonschema.DefaultNoGenerator,
			match:        jsonschema.GetFileFilter(jsonschema.NoGeneratorType),
			importValues: false,
			importCRDs:   true,
			objectCount:  1,
		},
		"skip crds": {
			opts: &helm.TemplateOpts{
				ChartName: "crds",
				RepoURL:   "./testdata",
				SkipCRDs:  true,
			},
			gen:          jsonschema.DefaultNoGenerator,
			match:        jsonschema.GetFileFilter(jsonschema.NoGeneratorType),
			importValues: false,
			importCRDs:   false,
			objectCount:  0,
		},
		"hooks only": {
			opts: &helm.TemplateOpts{
				ChartName: "test-hooks",
				RepoURL:   "./testdata",
				SkipHooks: false,
			},
			gen:          jsonschema.DefaultNoGenerator,
			match:        jsonschema.GetFileFilter(jsonschema.NoGeneratorType),
			importValues: false,
			importCRDs:   false,
			objectCount:  1,
		},
		"skip hooks": {
			opts: &helm.TemplateOpts{
				ChartName: "test-hooks",
				RepoURL:   "./testdata",
				SkipHooks: true,
			},
			gen:          jsonschema.DefaultNoGenerator,
			match:        jsonschema.GetFileFilter(jsonschema.NoGeneratorType),
			importValues: false,
			importCRDs:   false,
			objectCount:  0,
		},
	}
	for name, tc := range tcs {
		t.Run(name+"_chart", func(t *testing.T) {
			t.Parallel()

			c, err := helm.NewChart(helmtest.DefaultTestClient, helmrepo.DefaultManager, tc.opts)
			require.NoError(t, err)

			results, err := c.Template(t.Context())
			require.NoError(t, err)

			if tc.objectCount >= 0 {
				assert.Len(t, results, tc.objectCount)
			} else {
				assert.NotEmpty(t, results)
			}

			resultYAMLs, err := yaml.Marshal(results)
			require.NoError(t, err)

			if tc.objectCount != 0 {
				assert.NotEmpty(t, resultYAMLs)
			}

			// Write the results to testdata/got for debugging.
			gotDir := filepath.Join("testdata/got", t.Name())
			err = os.MkdirAll(gotDir, 0o700)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(gotDir, "output.yaml"), resultYAMLs, 0o600)
			require.NoError(t, err)
		})

		t.Run(name+"_chartfiles", func(t *testing.T) {
			t.Parallel()

			cf, err := helm.NewChartFiles(helmtest.DefaultTestClient, helmrepo.DefaultManager, maxSize, tc.opts)
			require.NoError(t, err)

			defer cf.Dispose()

			schema, err := cf.GetValuesJSONSchema(tc.gen, tc.match)
			require.NoError(t, err)

			crds, err := cf.GetCRDFiles(crd.DefaultFileGenerator, func(s string) bool {
				return filepath.Base(filepath.Dir(s)) == "crds" && filepath.Base(s) != "Chart.yaml" &&
					filepath.Ext(s) == ".yaml"
			})
			require.NoError(t, err)

			// Write the results to testdata/got for debugging.
			gotDir := filepath.Join("testdata/got", t.Name())
			err = os.MkdirAll(gotDir, 0o700)
			require.NoError(t, err)

			if tc.importValues {
				require.NotEmpty(t, schema)

				err = os.WriteFile(filepath.Join(gotDir, "values.schema.json"), schema, 0o600)
				require.NoError(t, err)
			}

			if tc.importCRDs {
				require.NotEmpty(t, crds)

				for _, crd := range crds {
					require.NotEmpty(t, crd)
				}
			}
		})
	}
}

func TestHelmChartTimeout(t *testing.T) {
	t.Parallel()

	c, err := helm.NewChart(helmtest.SlowTestClient, helmrepo.DefaultManager, &helm.TemplateOpts{
		ChartName:      "podinfo",
		TargetRevision: "6.7.1",
		RepoURL:        "https://stefanprodan.github.io/podinfo",
		Timeout:        100 * time.Millisecond,
	})
	require.NoError(t, err)

	_, err = c.Template(t.Context())
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestHelmChartAPIVersions(t *testing.T) {
	t.Parallel()

	t.Run("v1", func(t *testing.T) {
		t.Parallel()

		c, err := helm.NewChart(helmtest.DefaultTestClient, helmrepo.DefaultManager, &helm.TemplateOpts{
			ChartName:   "api-versions",
			RepoURL:     "./testdata",
			APIVersions: []string{"sample/v1"},
		})
		require.NoError(t, err)

		objs, err := c.Template(t.Context())
		require.NoError(t, err)
		require.Len(t, objs, 1)
		assert.Equal(t, "sample/v1", objs[0].GetAPIVersion())
	})

	t.Run("v2", func(t *testing.T) {
		t.Parallel()

		c, err := helm.NewChart(helmtest.DefaultTestClient, helmrepo.DefaultManager, &helm.TemplateOpts{
			ChartName:   "api-versions",
			RepoURL:     "./testdata",
			APIVersions: []string{"sample/v2"},
		})
		require.NoError(t, err)

		objs, err := c.Template(t.Context())
		require.NoError(t, err)
		require.Len(t, objs, 1)
		assert.Equal(t, "sample/v2", objs[0].GetAPIVersion())
	})
}

func BenchmarkHelmChart(b *testing.B) {
	c, err := helm.NewChart(helmtest.DefaultTestClient, helmrepo.DefaultManager, &helm.TemplateOpts{
		ChartName:      "podinfo",
		TargetRevision: "6.7.1",
		RepoURL:        "https://stefanprodan.github.io/podinfo",
	})
	require.NoError(b, err)

	_, err = c.Template(b.Context())
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := c.Template(b.Context())
			require.NoError(b, err)
		}
	})
}

func BenchmarkAppTemplateHelmChart(b *testing.B) {
	c, err := helm.NewChart(helmtest.DefaultTestClient, helmrepo.DefaultManager, &helm.TemplateOpts{
		ChartName:            "app-template",
		TargetRevision:       "3.6.0",
		RepoURL:              "https://bjw-s.github.io/helm-charts/",
		SkipSchemaValidation: true,
	})
	require.NoError(b, err)

	_, err = c.Template(b.Context())
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := c.Template(b.Context())
			require.NoError(b, err)
		}
	})
}
