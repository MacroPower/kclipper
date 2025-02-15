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
	"github.com/MacroPower/kclipper/pkg/pathutil"
)

const initialChartContents = `import helm

charts: helm.Charts = {}
`

func (c *ChartPkg) AddChart(chart *kclchart.ChartConfig) error {
	if err := c.Init(); err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}

	absBasePath, err := filepath.Abs(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	chartDir := path.Join(absBasePath, chart.GetSnakeCaseName())
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		return fmt.Errorf("failed to create charts directory: %w", err)
	}

	repoRoot, err := kclutil.FindRepoRoot(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to find repository root: %w", err)
	}

	pkgPath, err := kclutil.FindTopPkgRoot(repoRoot, c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to find package root: %w", err)
	}

	repoMgr := helmrepo.NewManager(helmrepo.WithAllowedPaths(pkgPath, repoRoot))

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

	if err := generateAndWriteChartKCL(&kclchart.Chart{ChartBase: chart.ChartBase}, chartDir); err != nil {
		return err
	}

	if err := generateAndWriteValuesSchemaFiles(chart, helmChart, pkgPath, repoRoot, chartDir); err != nil {
		return err
	}

	if chart.CRDPath != "" {
		crds, err := helmChart.GetCRDs(func(s string) bool {
			match, _ := filepath.Match(chart.CRDPath, s)

			return match
		})
		if err != nil {
			return fmt.Errorf("failed to get CRDs: %w", err)
		}

		if err := writeCRDFiles(crds, chartDir); err != nil {
			return err
		}
	}

	chartsFile := filepath.Join(c.BasePath, "charts.k")
	chartsSpec := kclutil.SpecPathJoin("charts", chart.GetSnakeCaseName())

	if err := c.updateFile(chart.ToAutomation(), chartsFile, initialChartContents, chartsSpec); err != nil {
		return fmt.Errorf("failed to update '%s': %w", chartsFile, err)
	}

	if _, err := kcl.FormatPath(c.BasePath); err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}

func generateAndWriteChartKCL(hc *kclchart.Chart, chartDir string) error {
	kclChart := &bytes.Buffer{}
	if err := hc.GenerateKCL(kclChart); err != nil {
		return fmt.Errorf("failed to generate chart.k: %w", err)
	}

	if err := os.WriteFile(path.Join(chartDir, "chart.k"), kclChart.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write chart.k: %w", err)
	}

	return nil
}

func generateAndWriteValuesSchemaFiles(
	chart *kclchart.ChartConfig, chartFiles *helm.ChartFiles, basePath, repoRoot, chartDir string,
) error {
	var (
		jsonSchemaBytes []byte
		err             error
	)

	switch chart.SchemaGenerator {
	case jsonschema.NoGeneratorType:
		break

	case jsonschema.URLGeneratorType, jsonschema.LocalPathGeneratorType:
		schemaPath, err := pathutil.ResolveFilePathOrURL(basePath, repoRoot, chart.SchemaPath, []string{"http", "https"})
		if err != nil {
			return fmt.Errorf("failed to resolve schema path: %w", err)
		}

		jsonSchemaBytes, err = jsonschema.DefaultReaderGenerator.FromPaths(schemaPath.String())
		if err != nil {
			return fmt.Errorf("failed to fetch schema from '%s': %w", schemaPath.String(), err)
		}

	case jsonschema.DefaultGeneratorType, jsonschema.AutoGeneratorType,
		jsonschema.ValueInferenceGeneratorType, jsonschema.ChartPathGeneratorType:
		fileMatcher := jsonschema.GetFileFilter(chart.SchemaGenerator)
		if chart.SchemaPath != "" {
			fileMatcher = func(f string) bool {
				return filePathsEqual(f, chart.SchemaPath)
			}
		}

		jsonSchemaBytes, err = chartFiles.GetValuesJSONSchema(jsonschema.GetGenerator(chart.SchemaGenerator), fileMatcher)
		if err != nil {
			return fmt.Errorf("failed to generate schema: %w", err)
		}
	}

	if len(jsonSchemaBytes) != 0 {
		if err := writeValuesSchemaFiles(jsonSchemaBytes, chartDir); err != nil {
			return err
		}
	}

	return nil
}

func writeValuesSchemaFiles(jsonSchema []byte, chartDir string) error {
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

func writeCRDFiles(crds [][]byte, chartDir string) error {
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
