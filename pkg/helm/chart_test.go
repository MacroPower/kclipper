package helm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclx/pkg/helm"
	"github.com/MacroPower/kclx/pkg/jsonschema"
)

func TestGetHelmValuesJsonSchema(t *testing.T) {
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
			},
			gen:   jsonschema.DefaultReaderGenerator,
			match: func(s string) bool { return s == "charts/common/values.schema.json" },
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := helm.NewChart(helm.DefaultClient, tc.opts)

			results, err := c.GetValuesJSONSchema(tc.gen, tc.match)
			require.NoError(t, err)
			require.NotNil(t, results)
		})
	}
}
