package helmmodels

import (
	"bytes"
	"fmt"

	"github.com/iancoleman/strcase"
	"kcl-lang.io/kcl-go/pkg/tools/gen"

	"github.com/MacroPower/kclx/pkg/jsonschema"
	"github.com/MacroPower/kclx/pkg/kclutil"
)

type ChartData struct {
	Charts map[string]ChartConfig `json:"charts"`
}

// ChartBase represents the KCL schema `helm.ChartBase`.
type ChartBase struct {
	// Chart is the Helm chart name.
	Chart string `json:"chart" jsonschema:"description=The Helm chart name."`
	// RepoURL is the URL of the Helm chart repository.
	RepoURL string `json:"repoURL" jsonschema:"description=The URL of the Helm chart repository."`
	// TargetRevision is the semver tag for the chart's version.
	TargetRevision string `json:"targetRevision" jsonschema:"description=The semver tag for the chart's version."`
	// ReleaseName is the Helm release name to use. If omitted it will use the chart name.
	ReleaseName string `json:"releaseName,omitempty" jsonschema:"-,description=The Helm release name to use. If omitted it will use the chart name."`
	// SkipCRDs will skip the custom resource definition installation step (--skip-crds).
	SkipCRDs bool `json:"skipCRDs,omitempty" jsonschema:"-,description=Skip the custom resource definition installation step."`
	// PassCredentials will pass credentials to all domains (--pass-credentials).
	PassCredentials bool `json:"passCredentials,omitempty" jsonschema:"-,description=Pass credentials to all domains."`
	// SchemaValidator is the validator to use for the Values schema.
	SchemaValidator jsonschema.ValidatorType `json:"schemaValidator,omitempty" jsonschema:"description=The validator to use for the Values schema."`
}

type ChartConfig struct {
	ChartBase
	// SchemaGenerator is the generator to use for the Values schema.
	SchemaGenerator jsonschema.GeneratorType `json:"schemaGenerator" jsonschema:"description=The generator to use for the Values schema."`
	// SchemaPath is the path to the schema to use.
	SchemaPath string `json:"schemaPath,omitempty" jsonschema:"description=The path to the JSONSchema to use when schemaGenerator = URL or PATH or LOCAL-PATH."`
}

func (c *ChartConfig) GetSnakeCaseName() string {
	return strcase.ToSnake(c.Chart)
}

func (c *ChartConfig) GenerateKCL(b *bytes.Buffer) error {
	r := jsonschema.NewReflector()
	js := r.Reflect(&ChartConfig{})
	if cv, ok := js.Properties.Get("schemaPath"); ok {
		cv.Default = c.SchemaPath
	}
	if cv, ok := js.Properties.Get("schemaGenerator"); ok {
		if c.SchemaGenerator != "" {
			cv.Default = c.SchemaGenerator
		} else {
			cv.Default = jsonschema.AutoGeneratorType
		}
		cv.Enum = jsonschema.GeneratorTypeEnum
	}
	if cv, ok := js.Properties.Get("schemaValidator"); ok {
		if c.SchemaValidator != "" {
			cv.Default = c.SchemaValidator
		} else {
			cv.Default = jsonschema.KCLValidatorType
		}
		cv.Enum = jsonschema.ValidatorTypeEnum
	}

	jsBytes, err := js.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal json schema: %w", err)
	}

	if err := kclutil.Gen.GenKcl(b, "settings", jsBytes, &gen.GenKclOptions{
		Mode:          gen.ModeJsonSchema,
		CastingOption: gen.OriginalName,
	}); err != nil {
		return fmt.Errorf("failed to generate kcl schema: %w", err)
	}

	return nil
}

type Chart struct {
	ChartBase
	// Values is the values to use for the chart.
	Values any `json:"values,omitempty" jsonschema:"description=The values to use for the chart."`
}

func (c *Chart) GetSnakeCaseName() string {
	return strcase.ToSnake(c.Chart)
}

func (c *Chart) GenerateKCL(b *bytes.Buffer) error {
	r := jsonschema.NewReflector()
	js := r.Reflect(&Chart{})
	if cv, ok := js.Properties.Get("chart"); ok {
		cv.Default = c.Chart
	}
	if cv, ok := js.Properties.Get("repoURL"); ok {
		cv.Default = c.RepoURL
	}
	if cv, ok := js.Properties.Get("targetRevision"); ok {
		cv.Default = c.TargetRevision
	}
	if cv, ok := js.Properties.Get("schemaValidator"); ok {
		if c.SchemaValidator != "" {
			cv.Default = c.SchemaValidator
		} else {
			cv.Default = jsonschema.KCLValidatorType
		}
		cv.Enum = jsonschema.ValidatorTypeEnum
	}

	jsBytes, err := js.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal json schema: %w", err)
	}

	if err := kclutil.Gen.GenKcl(b, "chart", jsBytes, &gen.GenKclOptions{
		Mode:          gen.ModeJsonSchema,
		CastingOption: gen.OriginalName,
	}); err != nil {
		return fmt.Errorf("failed to generate kcl schema: %w", err)
	}

	return nil
}
