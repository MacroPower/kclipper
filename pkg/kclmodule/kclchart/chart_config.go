package kclchart

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/iancoleman/strcase"

	"github.com/macropower/kclipper/pkg/crd"
	"github.com/macropower/kclipper/pkg/kclautomation"
	"github.com/macropower/kclipper/pkg/schema"
)

// ChartData holds a collection of chart configurations keyed by name.
type ChartData struct {
	Charts map[string]ChartConfig `json:"charts"`
}

// GetSortedKeys returns the chart keys in alphabetical order.
func (cd *ChartData) GetSortedKeys() []string {
	names := make([]string, 0, len(cd.Charts))
	for name := range cd.Charts {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

// GetByKey returns the [ChartConfig] for the given key and whether it exists.
func (cd *ChartData) GetByKey(k string) (ChartConfig, bool) {
	c, ok := cd.Charts[k]

	return c, ok
}

// FilterByName returns chart configurations whose Chart field matches name.
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

// GetSnakeCaseName returns the chart name converted to snake_case.
func (c *ChartConfig) GetSnakeCaseName() string {
	return strcase.ToSnake(c.Chart)
}

func (c *ChartConfig) Validate() error {
	var merr error

	if c.Chart == "" {
		merr = errors.Join(merr, errors.New("chart name is required"))
	}

	if c.RepoURL == "" {
		merr = errors.Join(merr, errors.New("repository URL is required"))
	}

	return merr
}

func (c *ChartConfig) GenerateKCL(w io.Writer) error {
	js, err := schema.Reflect[ChartConfig]()
	if err != nil {
		return fmt.Errorf("reflect schema: %w", err)
	}

	js.SetProperty("chart", schema.WithDefault(c.Chart))
	js.SetProperty("repoURL", schema.WithDefault(c.RepoURL))
	js.SetProperty("targetRevision", schema.WithDefault(c.TargetRevision))

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
		"schemaPath", c.SchemaPath != "",
		schema.WithDefault(c.SchemaPath),
	)
	js.SetOrRemoveProperty(
		"crdPath", len(c.CRDPaths) > 0,
		schema.WithDefault(c.CRDPaths),
	)
	js.SetOrRemoveProperty(
		"schemaValidator", c.SchemaValidator != schema.DefaultValidatorType,
		schema.WithDefault(c.SchemaValidator),
		schema.WithEnum(schema.ValidatorTypeEnum),
	)
	js.SetOrRemoveProperty(
		"schemaGenerator", c.SchemaGenerator != schema.DefaultGeneratorType,
		schema.WithDefault(c.SchemaGenerator),
		schema.WithEnum(schema.GeneratorTypeEnum),
	)
	js.SetOrRemoveProperty(
		"crdGenerator", c.CRDGenerator != crd.GeneratorTypeDefault,
		schema.WithDefault(c.CRDGenerator),
		schema.WithEnum(crd.GeneratorTypeEnum),
	)
	js.SetOrRemoveProperty(
		"repositories", len(c.Repositories) > 0,
		schema.WithDefault(c.Repositories),
		schema.WithType("null"),
		schema.WithNoContent(),
	)
	js.SetOrRemoveProperty(
		"values", c.Values != nil,
		schema.WithDefault(c.Values),
		schema.WithType("null"),
	)

	err = js.GenerateKCL(w, genOptFixChartRepo)
	if err != nil {
		return fmt.Errorf("convert JSON Schema to KCL Schema: %w", err)
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
