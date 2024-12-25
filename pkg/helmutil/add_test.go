package helmutil_test

import (
	"os"
	"path"
	"testing"

	"kcl-lang.io/kcl-go"

	helmchart "github.com/MacroPower/kclx/pkg/helm/chart"
	"github.com/MacroPower/kclx/pkg/helmutil"
)

const basePath = "./testdata"

func TestHelmChartAdd(t *testing.T) {
	t.Parallel()

	os.RemoveAll(basePath)
	err := helmutil.ChartInit(basePath)
	if err != nil {
		t.Fatal(err)
	}

	tcs := map[string]struct {
		chart      *helmchart.Chart
		schemaMode helmutil.SchemaMode
	}{
		"podinfo": {
			chart: &helmchart.Chart{
				Chart:          "podinfo",
				RepoURL:        "https://stefanprodan.github.io/podinfo",
				TargetRevision: "6.7.1",
			},
		},
		"app-template": {
			chart: &helmchart.Chart{
				Chart:          "app-template",
				RepoURL:        "https://bjw-s.github.io/helm-charts/",
				TargetRevision: "3.6.0",
			},
			schemaMode: helmutil.SchemaFromValues,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testPath := path.Join(basePath, "charts", tc.chart.Chart)

			ca := &helmutil.ChartAdd{
				BasePath:   path.Join(basePath, "charts"),
				SchemaMode: tc.schemaMode,
			}

			err := ca.Add(tc.chart.Chart, tc.chart.RepoURL, tc.chart.TargetRevision)
			if err != nil {
				t.Fatal(err)
			}

			results, err := kcl.LintPath([]string{testPath})
			if err != nil {
				t.Fatal(err)
			}

			if len(results) != 0 {
				t.Fatalf("expected no lint errors, got %v", results)
			}
		})
	}
}

func TestDefaultReplacement(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input string
		want  string
	}{
		"simple": {
			input: `    key: str = "default"`,
			want:  `    key: str`,
		},
		"two types": {
			input: `    key: str | int = 1`,
			want:  `    key: str | int`,
		},
		"many types": {
			input: `    key: str | int | {str:any} = {}`,
			want:  `    key: str | int | {str:any}`,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := helmutil.SchemaDefaultRegexp.ReplaceAllString(tc.input, "$1")
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
