package helmutil_test

import (
	"os"
	"path"
	"regexp"
	"testing"

	"kcl-lang.io/cli/pkg/options"
	"kcl-lang.io/kcl-go"

	helmchart "github.com/MacroPower/kclx/pkg/helm/chart"
	"github.com/MacroPower/kclx/pkg/helmutil"
)

const basePath = "testdata"

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

			ca := &helmutil.ChartAdd{
				BasePath:   basePath,
				SchemaMode: tc.schemaMode,
			}

			err := ca.Add(tc.chart.Chart, tc.chart.RepoURL, tc.chart.TargetRevision)
			if err != nil {
				t.Fatal(err)
			}

			depsOpt, err := options.LoadDepsFrom(path.Join(basePath, "charts"), true)
			if err != nil {
				t.Fatal(err)
			}
			results, err := kcl.Test(
				&kcl.TestOptions{
					PkgList:  []string{path.Join(basePath, "charts")},
					FailFast: true,
				},
				*depsOpt,
			)
			if err != nil {
				t.Fatal(err)
			}

			if len(results.Info) != 0 {
				t.Fatalf("expected no errors, got %d: %#v", len(results.Info), results)
			}
		})
	}
}

func TestDefaultReplacement(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input string
		want  string
		re    *regexp.Regexp
		repl  string
	}{
		"simple": {
			input: `    key: str = "default"`,
			want:  `    key: str`,
			re:    helmutil.SchemaDefaultRegexp,
			repl:  "$1",
		},
		"two types": {
			input: `    key: str | int = 1`,
			want:  `    key: str | int`,
			re:    helmutil.SchemaDefaultRegexp,
			repl:  "$1",
		},
		"many types": {
			input: `    key: str | int | {str:any} = {}`,
			want:  `    key: str | int | {str:any}`,
			re:    helmutil.SchemaDefaultRegexp,
			repl:  "$1",
		},
		"values": {
			input: `    values?: any`,
			want:  `    values?: x any`,
			re:    helmutil.SchemaValuesRegexp,
			repl:  "${1}x ${2}",
		},
		"values docs": {
			input: `    values : any, foobar`,
			want:  `    values : x any, foobar`,
			re:    helmutil.SchemaValuesRegexp,
			repl:  "${1}x ${2}",
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := tc.re.ReplaceAllString(tc.input, tc.repl)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
