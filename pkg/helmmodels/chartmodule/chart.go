package chartmodule

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"reflect"
	"regexp"

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
	if cv, ok := js.Properties.Get("chart"); ok {
		cv.Default = c.ChartBase.Chart
	}
	if cv, ok := js.Properties.Get("repoURL"); ok {
		cv.Default = c.ChartBase.RepoURL
	}
	if cv, ok := js.Properties.Get("targetRevision"); ok {
		cv.Default = c.ChartBase.TargetRevision
	}
	if cv, ok := js.Properties.Get("namespace"); ok {
		if c.Namespace != "" {
			cv.Default = c.ChartBase.Namespace
		} else {
			js.Properties.Delete("namespace")
		}
	}
	if cv, ok := js.Properties.Get("releaseName"); ok {
		if c.ReleaseName != "" {
			cv.Default = c.ChartBase.ReleaseName
		} else {
			js.Properties.Delete("releaseName")
		}
	}
	if cv, ok := js.Properties.Get("skipCRDs"); ok {
		if c.SkipCRDs {
			cv.Default = c.ChartBase.SkipCRDs
		} else {
			js.Properties.Delete("skipCRDs")
		}
	}
	if cv, ok := js.Properties.Get("passCredentials"); ok {
		if c.PassCredentials {
			cv.Default = c.ChartBase.PassCredentials
		} else {
			js.Properties.Delete("passCredentials")
		}
	}
	if cv, ok := js.Properties.Get("schemaValidator"); ok {
		if c.ChartBase.SchemaValidator != jsonschema.DefaultValidatorType {
			cv.Default = c.ChartBase.SchemaValidator
			cv.Enum = jsonschema.ValidatorTypeEnum
		} else {
			js.Properties.Delete("schemaValidator")
		}
	}
	if cv, ok := js.Properties.Get("schemaPath"); ok {
		if c.HelmChartConfig.SchemaPath != "" {
			cv.Default = c.HelmChartConfig.SchemaPath
		} else {
			js.Properties.Delete("schemaPath")
		}
	}
	if cv, ok := js.Properties.Get("schemaGenerator"); ok {
		if c.HelmChartConfig.SchemaGenerator != jsonschema.DefaultGeneratorType {
			cv.Default = c.HelmChartConfig.SchemaGenerator
			cv.Enum = jsonschema.GeneratorTypeEnum
		} else {
			js.Properties.Delete("schemaGenerator")
		}
	}

	err = jsonschema.ReflectedSchemaToKCL(js, w)
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
	js.Description = "All possible chart configuration, inheriting from `helm.Chart(helm.ChartBase)`."
	if cv, ok := js.Properties.Get("chart"); ok {
		cv.Default = c.ChartBase.Chart
	}
	if cv, ok := js.Properties.Get("repoURL"); ok {
		cv.Default = c.ChartBase.RepoURL
	}
	if cv, ok := js.Properties.Get("targetRevision"); ok {
		cv.Default = c.ChartBase.TargetRevision
	}
	if cv, ok := js.Properties.Get("namespace"); ok {
		if c.Namespace != "" {
			cv.Default = c.ChartBase.Namespace
		} else {
			js.Properties.Delete("namespace")
		}
	}
	if cv, ok := js.Properties.Get("releaseName"); ok {
		if c.ReleaseName != "" {
			cv.Default = c.ChartBase.ReleaseName
		} else {
			js.Properties.Delete("releaseName")
		}
	}
	if cv, ok := js.Properties.Get("skipCRDs"); ok {
		if c.SkipCRDs {
			cv.Default = c.ChartBase.SkipCRDs
		} else {
			js.Properties.Delete("skipCRDs")
		}
	}
	if cv, ok := js.Properties.Get("passCredentials"); ok {
		if c.PassCredentials {
			cv.Default = c.ChartBase.PassCredentials
		} else {
			js.Properties.Delete("passCredentials")
		}
	}
	if cv, ok := js.Properties.Get("schemaValidator"); ok {
		if c.ChartBase.SchemaValidator != jsonschema.DefaultValidatorType {
			cv.Default = c.ChartBase.SchemaValidator
			cv.Enum = jsonschema.ValidatorTypeEnum
		} else {
			js.Properties.Delete("schemaValidator")
		}
	}
	if cv, ok := js.Properties.Get("values"); ok {
		cv.Type = "null"
	}
	if _, ok := js.Properties.Get("valueFiles"); ok {
		js.Properties.Delete("valueFiles")
	}
	if _, ok := js.Properties.Get("postRenderer"); ok {
		js.Properties.Delete("postRenderer")
	}

	b := &bytes.Buffer{}
	err = jsonschema.ReflectedSchemaToKCL(js, b)
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
