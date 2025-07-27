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
	return strcase.ToSnake(c.Chart)
}

func (c *Chart) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}

	js := r.Reflect(reflect.TypeOf(Chart{}))
	js.Schema.Description = "All possible chart configuration, inheriting from `helm.Chart(helm.ChartBase)`."

	js.SetProperty("chart", jsonschema.WithDefault(c.Chart))
	js.SetProperty("repoURL", jsonschema.WithDefault(c.RepoURL))
	js.SetProperty("targetRevision", jsonschema.WithDefault(c.TargetRevision))
	js.SetProperty("values", jsonschema.WithDefault(c.Values), jsonschema.WithType("null"))

	js.SetOrRemoveProperty(
		"namespace", c.Namespace != "",
		jsonschema.WithDefault(c.Namespace),
	)
	js.SetOrRemoveProperty(
		"releaseName", c.ReleaseName != "",
		jsonschema.WithDefault(c.ReleaseName),
	)
	js.SetOrRemoveProperty(
		"skipCRDs", c.SkipCRDs,
		jsonschema.WithDefault(c.SkipCRDs),
	)
	js.SetOrRemoveProperty(
		"skipHooks", c.SkipHooks,
		jsonschema.WithDefault(c.SkipHooks),
	)
	js.SetOrRemoveProperty(
		"passCredentials", c.PassCredentials,
		jsonschema.WithDefault(c.PassCredentials),
	)
	js.SetOrRemoveProperty(
		"schemaValidator", c.SchemaValidator != jsonschema.DefaultValidatorType,
		jsonschema.WithDefault(c.SchemaValidator),
		jsonschema.WithEnum(jsonschema.ValidatorTypeEnum),
	)
	js.SetOrRemoveProperty(
		"repositories", len(c.Repositories) > 0,
		jsonschema.WithDefault(c.Repositories),
		jsonschema.WithType("null"),
		jsonschema.WithNoContent(),
	)

	js.RemoveProperty("valueFiles")
	js.RemoveProperty("postRenderer")

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b, genOptInheritHelmChart, genOptFixValues, genOptFixChartRepo)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	_, err = b.WriteTo(w)
	if err != nil {
		return fmt.Errorf("failed to write to KCL schema: %w", err)
	}

	return nil
}
