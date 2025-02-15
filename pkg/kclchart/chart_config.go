package kclchart

import (
	"fmt"
	"io"
	"reflect"
	"sort"

	"github.com/iancoleman/strcase"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/kclutil"
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
		"skipHooks", c.ChartBase.SkipHooks,
		jsonschema.WithDefault(c.ChartBase.SkipHooks),
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
	js.SetOrRemoveProperty(
		"repositories", len(c.ChartBase.Repositories) > 0,
		jsonschema.WithDefault(c.ChartBase.Repositories),
		jsonschema.WithType("null"),
		jsonschema.WithNoItems(),
	)

	err = js.GenerateKCL(w, genOptFixChartRepo)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	return nil
}

func (c *ChartConfig) ToAutomation() kclutil.Automation {
	return kclutil.Automation{
		"chart":           kclutil.NewString(c.Chart),
		"repoURL":         kclutil.NewString(c.RepoURL),
		"targetRevision":  kclutil.NewString(c.TargetRevision),
		"releaseName":     kclutil.NewString(c.ReleaseName),
		"namespace":       kclutil.NewString(c.Namespace),
		"skipCRDs":        kclutil.NewBool(c.SkipCRDs),
		"skipHooks":       kclutil.NewBool(c.SkipHooks),
		"passCredentials": kclutil.NewBool(c.PassCredentials),
		"schemaPath":      kclutil.NewString(c.SchemaPath),
		"crdPath":         kclutil.NewString(c.CRDPath),
		"schemaValidator": kclutil.NewString(string(c.SchemaValidator)),
		"schemaGenerator": kclutil.NewString(string(c.SchemaGenerator)),
	}
}
