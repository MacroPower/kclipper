package kclchart

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	"github.com/iancoleman/strcase"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

// All possible chart configuration, inheriting from `helm.Chart(helm.ChartBase)`.
type Chart struct {
	HelmChart
	ChartBase
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
	js.SetProperty("values", jsonschema.WithDefault(c.ChartBase.Values), jsonschema.WithType("null"))

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
		"skipHooks", c.ChartBase.SkipHooks,
		jsonschema.WithDefault(c.ChartBase.SkipHooks),
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
	js.SetOrRemoveProperty(
		"repositories", len(c.ChartBase.Repositories) > 0,
		jsonschema.WithDefault(c.ChartBase.Repositories),
		jsonschema.WithType("null"),
		jsonschema.WithNoItems(),
	)

	js.RemoveProperty("valueFiles")
	js.RemoveProperty("postRenderer")

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b, genOptInheritHelmChart, genOptFixValues, genOptFixChartRepo)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	if _, err := b.WriteTo(w); err != nil {
		return fmt.Errorf("failed to write to KCL schema: %w", err)
	}

	return nil
}
