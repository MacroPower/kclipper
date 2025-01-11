package pluginmodule

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"reflect"
	"regexp"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

var SchemaDefinitionRegexp = regexp.MustCompile(`schema\s+(\S+):\s*`)

// Represents attributes common in `helm.Chart` and `helm.ChartConfig`.
type ChartBase struct {
	// Helm chart name.
	Chart string `json:"chart"`
	// URL of the Helm chart repository.
	RepoURL string `json:"repoURL"`
	// Semver tag for the chart's version.
	TargetRevision string `json:"targetRevision"`
	// Helm release name to use. If omitted the chart name will be used.
	ReleaseName string `json:"releaseName,omitempty"`
	// Optional namespace to template with.
	Namespace string `json:"namespace,omitempty"`
	// Set to `True` to skip the custom resource definition installation step (Helm's `--skip-crds`).
	SkipCRDs bool `json:"skipCRDs,omitempty"`
	// Set to `True` to pass credentials to all domains (Helm's `--pass-credentials`).
	PassCredentials bool `json:"passCredentials,omitempty"`
	// Validator to use for the Values schema.
	SchemaValidator jsonschema.ValidatorType `json:"schemaValidator,omitempty"`
}

func (c *ChartBase) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}
	js := r.Reflect(reflect.TypeOf(ChartBase{}))

	js.SetProperty("schemaValidator", jsonschema.WithEnum(jsonschema.ValidatorTypeEnum))

	err = js.GenerateKCL(w)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	return nil
}

// Defines a Helm chart.
type Chart struct {
	// Helm values to be passed to Helm template. These take precedence over valueFiles.
	Values any `json:"values,omitempty"`
	// Helm value files to be passed to Helm template.
	ValueFiles []string `json:"valueFiles,omitempty"`
	// Lambda function to modify the Helm template output. Evaluated for each resource in the Helm template output.
	PostRenderer any `json:"postRenderer,omitempty"`
}

var SchemaPostRendererRegexp = regexp.MustCompile(`(\s+postRenderer\??\s*:\s+)any(.*)`)

const PostRendererKCLType string = "({str:}) -> {str:}"

func (c *Chart) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}
	js := r.Reflect(reflect.TypeOf(Chart{}))

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	nb := &bytes.Buffer{}
	scanner := bufio.NewScanner(b)
	for scanner.Scan() {
		line := scanner.Text()
		line = inheritChartBase(line)
		if SchemaPostRendererRegexp.MatchString(line) {
			line = SchemaPostRendererRegexp.ReplaceAllString(line, "${1}"+PostRendererKCLType+"${2}")
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

// Configuration that can be defined in `charts.k`, in addition to those
// specified in `helm.ChartBase`.
type ChartConfig struct {
	// Schema generator to use for the Values schema.
	SchemaGenerator jsonschema.GeneratorType `json:"schemaGenerator,omitempty"`
	// Path to the schema to use, when relevant for the selected schemaGenerator.
	SchemaPath string `json:"schemaPath,omitempty"`
	// Path to any CRDs to import as schemas. Glob patterns are supported.
	CRDPath string `json:"crdPath,omitempty"`
}

func (c *ChartConfig) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}
	js := r.Reflect(reflect.TypeOf(ChartConfig{}))

	js.SetProperty("schemaGenerator", jsonschema.WithEnum(jsonschema.GeneratorTypeEnum))

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	nb := &bytes.Buffer{}
	scanner := bufio.NewScanner(b)
	for scanner.Scan() {
		line := scanner.Text()
		line = inheritChartBase(line)
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

func newSchemaReflector() (*jsonschema.Reflector, error) {
	r := jsonschema.NewReflector()
	err := r.AddGoComments("github.com/MacroPower/kclipper", "./pkg/helmmodels/pluginmodule")
	if err != nil {
		return nil, fmt.Errorf("failed to add go comments: %w", err)
	}

	return r, nil
}

func inheritChartBase(line string) string {
	if SchemaDefinitionRegexp.MatchString(line) {
		return SchemaDefinitionRegexp.ReplaceAllString(line, "schema ${1}(ChartBase):")
	}
	return line
}
