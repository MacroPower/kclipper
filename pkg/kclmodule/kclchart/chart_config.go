package kclchart

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"

	"github.com/hashicorp/go-multierror"
	"github.com/iancoleman/strcase"

	"github.com/MacroPower/kclipper/pkg/crd"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/kclautomation"
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

func (cd *ChartData) GetByKey(k string) (ChartConfig, bool) {
	c, ok := cd.Charts[k]

	return c, ok
}

func (cd *ChartData) FilterByName(name string) map[string]ChartConfig {
	m := map[string]ChartConfig{}
	for k := range cd.Charts {
		if cd.Charts[k].Chart == name {
			m[k] = cd.Charts[k]
		}
	}

	return m
}

// All possible chart configuration that can be defined in `charts.k`,
// inheriting from `helm.ChartConfig(helm.ChartBase)`.
type ChartConfig struct {
	HelmChartConfig
	ChartBase
}

func (c *ChartConfig) GetSnakeCaseName() string {
	return strcase.ToSnake(c.Chart)
}

func (c *ChartConfig) Validate() error {
	var merr error

	if c.Chart == "" {
		merr = multierror.Append(merr, errors.New("chart name is required"))
	}

	if c.RepoURL == "" {
		merr = multierror.Append(merr, errors.New("repository URL is required"))
	}

	return merr
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
		"crdPath", len(c.HelmChartConfig.CRDPaths) > 0,
		jsonschema.WithDefault(c.HelmChartConfig.CRDPaths),
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
		"crdGenerator", c.HelmChartConfig.CRDGenerator != crd.GeneratorTypeDefault,
		jsonschema.WithDefault(c.HelmChartConfig.CRDGenerator),
		jsonschema.WithEnum(crd.GeneratorTypeEnum),
	)
	js.SetOrRemoveProperty(
		"repositories", len(c.ChartBase.Repositories) > 0,
		jsonschema.WithDefault(c.ChartBase.Repositories),
		jsonschema.WithType("null"),
		jsonschema.WithNoContent(),
	)
	js.SetOrRemoveProperty(
		"values", c.ChartBase.Values != nil,
		jsonschema.WithDefault(c.ChartBase.Values),
		jsonschema.WithType("null"),
	)

	err = js.GenerateKCL(w, genOptFixChartRepo)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	return nil
}

func (c *ChartConfig) ToAutomation() kclautomation.Automation {
	return kclautomation.Automation{
		"chart":           kclautomation.NewString(c.Chart),
		"repoURL":         kclautomation.NewString(c.RepoURL),
		"targetRevision":  kclautomation.NewString(c.TargetRevision),
		"releaseName":     kclautomation.NewString(c.ReleaseName),
		"namespace":       kclautomation.NewString(c.Namespace),
		"skipCRDs":        kclautomation.NewBool(c.SkipCRDs),
		"skipHooks":       kclautomation.NewBool(c.SkipHooks),
		"passCredentials": kclautomation.NewBool(c.PassCredentials),
		"schemaPath":      kclautomation.NewString(c.SchemaPath),
		"schemaValidator": kclautomation.NewString(string(c.SchemaValidator)),
		"schemaGenerator": kclautomation.NewString(string(c.SchemaGenerator)),
		"crdGenerator":    kclautomation.NewString(string(c.CRDGenerator)),
	}
}
