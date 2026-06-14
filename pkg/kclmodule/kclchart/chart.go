package kclchart

import (
	"fmt"
	"io"

	"github.com/iancoleman/strcase"

	"github.com/macropower/kclipper/pkg/schema"
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
	js, err := schema.Reflect[Chart]()
	if err != nil {
		return fmt.Errorf("reflect schema: %w", err)
	}

	js.Schema.Description = "All possible chart configuration, inheriting from `helm.Chart(helm.ChartBase)`."

	js.SetProperty("chart", schema.WithDefault(c.Chart))
	js.SetProperty("repoURL", schema.WithDefault(c.RepoURL))
	js.SetProperty("targetRevision", schema.WithDefault(c.TargetRevision))
	js.SetProperty("values", schema.WithDefault(c.Values), schema.WithType("null"))

	js.SetOrRemoveProperty(
		"namespace", c.Namespace != "",
		schema.WithDefault(c.Namespace),
	)
	js.SetOrRemoveProperty(
		"releaseName", c.ReleaseName != "",
		schema.WithDefault(c.ReleaseName),
	)
	js.SetOrRemoveProperty(
		"skipCRDs", c.SkipCRDs,
		schema.WithDefault(c.SkipCRDs),
	)
	js.SetOrRemoveProperty(
		"skipHooks", c.SkipHooks,
		schema.WithDefault(c.SkipHooks),
	)
	js.SetOrRemoveProperty(
		"passCredentials", c.PassCredentials,
		schema.WithDefault(c.PassCredentials),
	)
	js.SetOrRemoveProperty(
		"schemaValidator", c.SchemaValidator != schema.DefaultValidatorType,
		schema.WithDefault(c.SchemaValidator),
		schema.WithEnum(schema.ValidatorTypeEnum),
	)
	js.SetOrRemoveProperty(
		"repositories", len(c.Repositories) > 0,
		schema.WithDefault(c.Repositories),
		schema.WithType("null"),
		schema.WithNoContent(),
	)

	js.RemoveProperty("valueFiles")
	js.RemoveProperty("postRenderer")

	err = js.GenerateKCL(w, genOptInheritHelmChart, genOptFixValues, genOptFixChartRepo)
	if err != nil {
		return fmt.Errorf("convert JSON Schema to KCL schema: %w", err)
	}

	return nil
}
