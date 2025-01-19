package helmutil

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/kclchart"
	"github.com/MacroPower/kclipper/pkg/kclutil"
)

const initialChartContents = `import helm

charts: helm.Charts = {}
`

func (c *ChartPkg) Add(chart *kclchart.ChartConfig) error {
	chartDir := path.Join(c.BasePath, chart.GetSnakeCaseName())

	if err := c.Init(); err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		return fmt.Errorf("failed to create charts directory: %w", err)
	}

	repoMgr := helmrepo.NewManager()
	for _, repo := range chart.Repositories {
		hr, err := repo.GetHelmRepo()
		if err != nil {
			return fmt.Errorf("failed to add Helm repository: %w", err)
		}
		if err := repoMgr.Add(hr); err != nil {
			return fmt.Errorf("failed to add Helm repository: %w", err)
		}
	}

	helmChart, err := helm.NewChartFiles(c.Client, repoMgr, helm.TemplateOpts{
		ChartName:       chart.Chart,
		TargetRevision:  chart.TargetRevision,
		RepoURL:         chart.RepoURL,
		SkipCRDs:        chart.SkipCRDs,
		PassCredentials: chart.PassCredentials,
	})
	if err != nil {
		return fmt.Errorf("failed to create chart handler for '%s': %w", chart.Chart, err)
	}
	defer helmChart.Dispose()

	if err := c.generateAndWriteChartKCL(&kclchart.Chart{ChartBase: chart.ChartBase}, chartDir); err != nil {
		return err
	}

	var jsonSchemaBytes []byte
	switch chart.SchemaGenerator {
	case jsonschema.NoGeneratorType:
		break
	case jsonschema.URLGeneratorType, jsonschema.LocalPathGeneratorType:
		jsonSchemaBytes, err = jsonschema.DefaultReaderGenerator.FromPaths(chart.SchemaPath)
		if err != nil {
			return fmt.Errorf("failed to fetch schema from %s: %w", chart.SchemaPath, err)
		}
	case jsonschema.DefaultGeneratorType, jsonschema.AutoGeneratorType,
		jsonschema.ValueInferenceGeneratorType, jsonschema.ChartPathGeneratorType:
		fileMatcher := jsonschema.GetFileFilter(chart.SchemaGenerator)
		if chart.SchemaPath != "" {
			fileMatcher = func(f string) bool {
				return filePathsEqual(f, chart.SchemaPath)
			}
		}
		jsonSchemaBytes, err = helmChart.GetValuesJSONSchema(jsonschema.GetGenerator(chart.SchemaGenerator), fileMatcher)
		if err != nil {
			return fmt.Errorf("failed to generate schema: %w", err)
		}
	}

	if len(jsonSchemaBytes) != 0 {
		if err := c.writeValuesSchemaFiles(jsonSchemaBytes, chartDir); err != nil {
			return err
		}
	}

	if chart.CRDPath != "" {
		crds, err := helmChart.GetCRDs(func(s string) bool {
			match, _ := filepath.Match(chart.CRDPath, s)
			return match
		})
		if err != nil {
			return fmt.Errorf("failed to get CRDs: %w", err)
		}

		if err := c.writeCRDFiles(crds, chartDir); err != nil {
			return err
		}
	}

	chartsFile := filepath.Join(c.BasePath, "charts.k")
	chartsSpec := kclutil.SpecPathJoin("charts", chart.GetSnakeCaseName())
	err = c.updateFile(chart.ToAutomation(), chartsFile, initialChartContents, chartsSpec)
	if err != nil {
		return fmt.Errorf("failed to update '%s': %w", chartsFile, err)
	}

	_, err = kcl.FormatPath(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}

func (c *ChartPkg) generateAndWriteChartKCL(hc *kclchart.Chart, chartDir string) error {
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

func filePathsEqual(f1, f2 string) bool {
	return filepath.Clean(f1) == filepath.Clean(f2)
}
