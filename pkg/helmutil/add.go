package helmutil

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"

	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/helm"
	helmmodels "github.com/MacroPower/kclipper/pkg/helmmodels/chartmodule"
	"github.com/MacroPower/kclipper/pkg/helmmodels/pluginmodule"
	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/kclutil"
)

const initialMainContents = `import helm

charts: helm.Charts = {}
`

func (c *ChartPkg) Add(
	chart, repoURL, targetRevision, schemaPath, crdPath string,
	genType jsonschema.GeneratorType,
	validateType jsonschema.ValidatorType,
	helmRepos []pluginmodule.ChartRepo,
) error {
	hc := helmmodels.Chart{
		ChartBase: helmmodels.ChartBase{
			Chart:           chart,
			RepoURL:         repoURL,
			TargetRevision:  targetRevision,
			SchemaValidator: validateType,
			Repositories:    helmRepos,
		},
	}

	chartDir := path.Join(c.BasePath, hc.GetSnakeCaseName())

	if err := c.Init(); err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		return fmt.Errorf("failed to create charts directory: %w", err)
	}

	repoMgr := helmrepo.NewManager()
	for _, repo := range helmRepos {
		hr, err := repo.GetHelmRepo()
		if err != nil {
			return fmt.Errorf("failed to add Helm repository: %w", err)
		}
		if err := repoMgr.Add(hr); err != nil {
			return fmt.Errorf("failed to add Helm repository: %w", err)
		}
	}

	helmChart, err := helm.NewChartFiles(c.Client, repoMgr, helm.TemplateOpts{
		ChartName:      chart,
		TargetRevision: targetRevision,
		RepoURL:        repoURL,
	})
	if err != nil {
		return fmt.Errorf("failed to create chart handler for '%s': %w", chart, err)
	}
	defer helmChart.Dispose()

	if err := c.generateAndWriteChartKCL(hc, chartDir); err != nil {
		return err
	}

	var jsonSchemaBytes []byte
	switch genType {
	case jsonschema.NoGeneratorType:
		break
	case jsonschema.URLGeneratorType, jsonschema.LocalPathGeneratorType:
		jsonSchemaBytes, err = jsonschema.DefaultReaderGenerator.FromPaths(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to fetch schema from %s: %w", schemaPath, err)
		}
	case jsonschema.DefaultGeneratorType, jsonschema.AutoGeneratorType,
		jsonschema.ValueInferenceGeneratorType, jsonschema.ChartPathGeneratorType:
		fileMatcher := jsonschema.GetFileFilter(genType)
		if schemaPath != "" {
			fileMatcher = func(f string) bool {
				return filePathsEqual(f, schemaPath)
			}
		}
		jsonSchemaBytes, err = helmChart.GetValuesJSONSchema(jsonschema.GetGenerator(genType), fileMatcher)
		if err != nil {
			return fmt.Errorf("failed to generate schema: %w", err)
		}
	}

	if len(jsonSchemaBytes) != 0 {
		if err := c.writeValuesSchemaFiles(jsonSchemaBytes, chartDir); err != nil {
			return err
		}
	}

	if crdPath != "" {
		crds, err := helmChart.GetCRDs(func(s string) bool {
			match, _ := filepath.Match(crdPath, s)
			return match
		})
		if err != nil {
			return fmt.Errorf("failed to get CRDs: %w", err)
		}

		if err := c.writeCRDFiles(crds, chartDir); err != nil {
			return err
		}
	}

	chartConfig := map[string]string{
		"chart":           chart,
		"repoURL":         repoURL,
		"targetRevision":  targetRevision,
		"schemaGenerator": string(genType),
		"schemaPath":      schemaPath,
		"crdPath":         crdPath,
		"schemaValidator": string(validateType),
	}
	if err := c.updateChartsFile(c.BasePath, hc.GetSnakeCaseName(), chartConfig); err != nil {
		return err
	}

	_, err = kcl.FormatPath(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}

func (c *ChartPkg) generateAndWriteChartKCL(hc helmmodels.Chart, chartDir string) error {
	kclChart := &bytes.Buffer{}
	if err := hc.GenerateKCL(kclChart); err != nil {
		return fmt.Errorf("failed to generate chart.k: %w", err)
	}
	if err := os.WriteFile(path.Join(chartDir, "chart.k"), kclChart.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write chart.k: %w", err)
	}
	return nil
}

func (c *ChartPkg) writeValuesSchemaFiles(jsonSchema []byte, chartDir string) error {
	if err := os.WriteFile(path.Join(chartDir, "values.schema.json"), jsonSchema, 0o600); err != nil {
		return fmt.Errorf("failed to write values.schema.json: %w", err)
	}

	kclSchema, err := jsonschema.ConvertToKCLSchema(jsonSchema, true)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	if err := os.WriteFile(path.Join(chartDir, "values.schema.k"), kclSchema, 0o600); err != nil {
		return fmt.Errorf("failed to write values.schema.k: %w", err)
	}
	return nil
}

func (c *ChartPkg) writeCRDFiles(crds [][]byte, chartDir string) error {
	for _, crd := range crds {
		err := kclutil.GenerateKCLFromCRD(crd, chartDir)
		if err != nil {
			return fmt.Errorf("failed to generate KCL from CRD: %w", err)
		}
	}
	return nil
}

func (c *ChartPkg) updateChartsFile(vendorDir, chartKey string, chartConfig map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	chartsFile := path.Join(vendorDir, "charts.k")
	if !fileExists(chartsFile) {
		if err := os.WriteFile(chartsFile, []byte(initialMainContents), 0o600); err != nil {
			return fmt.Errorf("failed to write '%s': %w", chartsFile, err)
		}
	}
	imports := []string{"helm"}
	specs := sort.StringSlice{}
	for k, v := range chartConfig {
		if k == "" {
			return fmt.Errorf("invalid key in chart config: %#v", chartConfig)
		}
		if v == "" {
			continue
		}
		specs = append(specs, fmt.Sprintf(`charts.%s.%s="%s"`, chartKey, k, v))
	}
	specs.Sort()
	_, err := kcl.OverrideFile(chartsFile, specs, imports)
	if err != nil {
		return fmt.Errorf("failed to update '%s': %w", chartsFile, err)
	}
	return nil
}

func filePathsEqual(f1, f2 string) bool {
	return filepath.Clean(f1) == filepath.Clean(f2)
}
