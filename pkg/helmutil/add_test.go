package helmutil_test

import (
	"os"
	"path"
	"regexp"
	"testing"

	"kcl-lang.io/cli/pkg/options"
	"kcl-lang.io/kcl-go"

	helmchart "github.com/MacroPower/kclx/pkg/helm/models"
	"github.com/MacroPower/kclx/pkg/helmutil"
	"github.com/MacroPower/kclx/pkg/jsonschema"
)

const (
	addBasePath = "testdata/add"
)

func TestHelmChartAdd(t *testing.T) {
	t.Parallel()

	chartPath := path.Join(addBasePath, "charts")
	os.RemoveAll(chartPath)

	ca := helmutil.NewChartPkg(chartPath)

	if err := ca.Init(); err != nil {
		t.Fatal(err)
	}

	tcs := map[string]struct {
		chart *helmchart.ChartConfig
	}{
		"podinfo": {
			chart: &helmchart.ChartConfig{
				ChartBase: helmchart.ChartBase{
					Chart:          "podinfo",
					RepoURL:        "https://stefanprodan.github.io/podinfo",
					TargetRevision: "6.7.1",
				},
				SchemaGenerator: jsonschema.AutoGeneratorType,
			},
		},
		"app-template": {
			chart: &helmchart.ChartConfig{
				ChartBase: helmchart.ChartBase{
					Chart:          "app-template",
					RepoURL:        "https://bjw-s.github.io/helm-charts/",
					TargetRevision: "3.6.0",
				},
				SchemaGenerator: jsonschema.ValueInferenceGeneratorType,
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := ca.Add(tc.chart.Chart, tc.chart.RepoURL, tc.chart.TargetRevision,
				tc.chart.SchemaPath, tc.chart.SchemaGenerator)
			if err != nil {
				t.Fatal(err)
			}

			depsOpt, err := options.LoadDepsFrom(chartPath, true)
			if err != nil {
				t.Fatal(err)
			}
			results, err := kcl.Test(
				&kcl.TestOptions{
					PkgList:  []string{chartPath},
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
