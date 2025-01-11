package chartmodule

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"

	"github.com/iancoleman/strcase"

	"github.com/MacroPower/kclipper/pkg/helmmodels/pluginmodule"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

var (
	SchemaDefinitionRegexp = regexp.MustCompile(`schema\s+(\S+):\s*`)
	SchemaValuesRegexp     = regexp.MustCompile(`(\s+values\??\s*:\s+)(.*)`)
)

type ChartData struct {
	Charts map[string]ChartConfig `json:"charts"`
}

// GetSortedKeys returns the chart keys in alphabetical order.
func (cd *ChartData) GetSortedKeys() []string {
	names := make([]string, 0, len(cd.Charts))
	for name := range cd.Charts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type (
	ChartBase       pluginmodule.ChartBase
	HelmChartConfig pluginmodule.ChartConfig
	HelmChart       pluginmodule.Chart
)

// All possible chart configuration that can be defined in `charts.k`,
// inheriting from `helm.ChartConfig(helm.ChartBase)`.
type ChartConfig struct {
	ChartBase       `json:",inline"`
	HelmChartConfig `json:",inline"`
}

func (c *ChartConfig) GetSnakeCaseName() string {
	return strcase.ToSnake(c.Chart)
}

func (c *ChartConfig) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}
	js := r.Reflect(reflect.TypeOf(ChartConfig{}))

	js.SetProperty("chart", jsonschema.WithDefault(c.ChartBase.Chart))
	js.SetProperty("repoURL", jsonschema.WithDefault(c.ChartBase.RepoURL))
	js.SetProperty("targetRevision", jsonschema.WithDefault(c.ChartBase.TargetRevision))

	js.SetOrRemoveProperty(
		"namespace", c.ChartBase.Namespace != "",
		jsonschema.WithDefault(c.ChartBase.Namespace),
	)
	js.SetOrRemoveProperty(
		"releaseName", c.ChartBase.ReleaseName != "",
		jsonschema.WithDefault(c.ChartBase.ReleaseName),
	)
	js.SetOrRemoveProperty(
		"skipCRDs", c.ChartBase.SkipCRDs,
		jsonschema.WithDefault(c.ChartBase.SkipCRDs),
	)
	js.SetOrRemoveProperty(
		"passCredentials", c.ChartBase.PassCredentials,
		jsonschema.WithDefault(c.ChartBase.PassCredentials),
	)
	js.SetOrRemoveProperty(
		"schemaPath", c.HelmChartConfig.SchemaPath != "",
		jsonschema.WithDefault(c.HelmChartConfig.SchemaPath),
	)
	js.SetOrRemoveProperty(
		"crdPath", c.HelmChartConfig.CRDPath != "",
		jsonschema.WithDefault(c.HelmChartConfig.CRDPath),
	)
	js.SetOrRemoveProperty(
		"schemaValidator", c.ChartBase.SchemaValidator != jsonschema.DefaultValidatorType,
		jsonschema.WithDefault(c.ChartBase.SchemaValidator),
		jsonschema.WithEnum(jsonschema.ValidatorTypeEnum),
	)
	js.SetOrRemoveProperty(
		"schemaGenerator", c.HelmChartConfig.SchemaGenerator != jsonschema.DefaultGeneratorType,
		jsonschema.WithDefault(c.HelmChartConfig.SchemaGenerator),
		jsonschema.WithEnum(jsonschema.GeneratorTypeEnum),
	)

	err = js.GenerateKCL(w)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	return nil
}

// All possible chart configuration, inheriting from `helm.Chart(helm.ChartBase)`.
type Chart struct {
	ChartBase `json:",inline"`
	HelmChart `json:",inline"`
}

func (c *Chart) GetSnakeCaseName() string {
	return strcase.ToSnake(c.ChartBase.Chart)
}

func (c *Chart) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}
	js := r.Reflect(reflect.TypeOf(Chart{}))
	js.Schema.Description = "All possible chart configuration, inheriting from `helm.Chart(helm.ChartBase)`."

	js.SetProperty("chart", jsonschema.WithDefault(c.ChartBase.Chart))
	js.SetProperty("repoURL", jsonschema.WithDefault(c.ChartBase.RepoURL))
	js.SetProperty("targetRevision", jsonschema.WithDefault(c.ChartBase.TargetRevision))
	js.SetProperty("values", jsonschema.WithType("null"))

	js.SetOrRemoveProperty(
		"namespace", c.ChartBase.Namespace != "",
		jsonschema.WithDefault(c.ChartBase.Namespace),
	)
	js.SetOrRemoveProperty(
		"releaseName", c.ChartBase.ReleaseName != "",
		jsonschema.WithDefault(c.ChartBase.ReleaseName),
	)
	js.SetOrRemoveProperty(
		"skipCRDs", c.ChartBase.SkipCRDs,
		jsonschema.WithDefault(c.ChartBase.SkipCRDs),
	)
	js.SetOrRemoveProperty(
		"passCredentials", c.ChartBase.PassCredentials,
		jsonschema.WithDefault(c.ChartBase.PassCredentials),
	)
	js.SetOrRemoveProperty(
		"schemaValidator", c.ChartBase.SchemaValidator != jsonschema.DefaultValidatorType,
		jsonschema.WithDefault(c.ChartBase.SchemaValidator),
		jsonschema.WithEnum(jsonschema.ValidatorTypeEnum),
	)

	js.RemoveProperty("valueFiles")
	js.RemoveProperty("postRenderer")

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}
	nb := &bytes.Buffer{}
	scanner := bufio.NewScanner(b)
	for scanner.Scan() {
		line := scanner.Text()
		line = inheritHelmChart(line)
		if SchemaValuesRegexp.MatchString(line) {
			line = SchemaValuesRegexp.ReplaceAllString(line, "${1}Values | ${2}")
		}
		nb.WriteString(line + "\n")
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan kcl schema: %w", err)
	}
	if _, err := nb.WriteTo(w); err != nil {
		return fmt.Errorf("failed to write to KCL schema: %w", err)
	}

	return nil
}

//nolint:unparam
func newSchemaReflector() (*jsonschema.Reflector, error) {
	r := jsonschema.NewReflector()

	return r, nil
}

func inheritHelmChart(line string) string {
	if SchemaDefinitionRegexp.MatchString(line) {
		return SchemaDefinitionRegexp.ReplaceAllString(line, "import helm\n\nschema ${1}(helm.Chart):")
	}
	return line
}
