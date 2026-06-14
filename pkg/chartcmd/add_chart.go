package chartcmd

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"kcl-lang.io/kcl-go"

	"github.com/macropower/kclipper/pkg/crd"
	"github.com/macropower/kclipper/pkg/helm"
	"github.com/macropower/kclipper/pkg/helmrepo"
	"github.com/macropower/kclipper/pkg/kclautomation"
	"github.com/macropower/kclipper/pkg/kclmodule/kclchart"
	"github.com/macropower/kclipper/pkg/kube"
	"github.com/macropower/kclipper/pkg/paths"
	"github.com/macropower/kclipper/pkg/schema"
)

const initialChartContents = `import helm

charts: helm.Charts = {}
`

// AddChart adds a new chart to the chart package.
func (c *KCLPackage) AddChart(key string, chart *kclchart.ChartConfig) error {
	err := chart.Validate()
	if err != nil {
		return fmt.Errorf("validate chart config: %w", err)
	}

	logger := slog.With(
		slog.String("cmd", "chart_add"),
		slog.String("chart_key", key),
		slog.String("chart", chart.Chart),
	)

	logger.Info("check init before add")

	err = c.Init()
	if err != nil {
		return fmt.Errorf("init before add: %w", err)
	}

	chartDir, err := c.addChartDir(key, logger)
	if err != nil {
		return err
	}

	helmChart, err := c.setupHelmChart(chart, logger)
	if err != nil {
		return err
	}
	defer helmChart.Dispose()

	err = generateAndWriteChartKCL(&kclchart.Chart{ChartBase: chart.ChartBase}, chartDir, logger)
	if err != nil {
		return err
	}

	jsonSchemaBytes, err := c.getValuesJSONSchema(chart, helmChart, logger)
	if err != nil {
		return err
	}

	if len(jsonSchemaBytes) != 0 {
		err := writeValuesSchemaFiles(jsonSchemaBytes, chartDir)
		if err != nil {
			return err
		}
	}

	crds, err := c.getCRDs(chart, helmChart, logger)
	if err != nil {
		return err
	}

	if len(crds) > 0 {
		err := c.writeCRDFiles(crds, chartDir)
		if err != nil {
			return err
		}
	}

	err = c.updateChartsFile(key, chart, logger)
	if err != nil {
		return err
	}

	logger.Info("formatting kcl files", slog.String("path", chartDir))

	_, err = kcl.FormatPath(filepath.Join(chartDir, "..."))
	if err != nil {
		return fmt.Errorf("format kcl files: %w", err)
	}

	return nil
}

// addChartDir prepares the necessary directories for the chart.
func (c *KCLPackage) addChartDir(key string, logger *slog.Logger) (string, error) {
	chartDir := path.Join(c.absBasePath, key)
	logger.Debug("ensure chart directory", slog.String("path", chartDir))

	err := os.MkdirAll(chartDir, 0o750)
	if err != nil {
		return "", fmt.Errorf("create chart directory: %w", err)
	}

	logger.Debug("ensured chart directory", slog.String("path", chartDir))

	return chartDir, nil
}

// setupHelmChart sets up the helm repositories and creates a chart.
func (c *KCLPackage) setupHelmChart(chart *kclchart.ChartConfig, logger *slog.Logger) (*helm.ChartFiles, error) {
	logger.Info("loading helm repositories")

	repoMgr := helmrepo.NewManager(helmrepo.WithAllowedPaths(c.pkgPath, c.repoRoot))

	// Add repositories.
	for _, repo := range chart.Repositories {
		logger.Debug("adding helm repository",
			slog.String("name", repo.Name),
			slog.String("url", repo.URL),
		)

		hr, err := repo.GetHelmRepo()
		if err != nil {
			return nil, fmt.Errorf("get helm repository: %w", err)
		}

		err = repoMgr.Add(hr)
		if err != nil {
			return nil, fmt.Errorf("add helm repository: %w", err)
		}
	}

	// Prepare chart values.
	chartValues := map[string]any{}
	if chart.Values != nil {
		var ok bool

		chartValues, ok = chart.Values.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid values type: %T", chart.Values)
		}
	}

	// Load helm chart.
	logger.Info("loading helm chart files")

	helmChart, err := helm.NewChartFiles(c.Client, repoMgr, c.MaxExtractSize, &helm.TemplateOpts{
		ChartName:       chart.Chart,
		TargetRevision:  chart.TargetRevision,
		RepoURL:         chart.RepoURL,
		SkipCRDs:        chart.SkipCRDs,
		PassCredentials: chart.PassCredentials,
		ValuesObject:    chartValues,
		// KCL validates values against the generated schema, so Helm-side
		// validation (which can load remote JSON Schema refs) is redundant.
		SkipSchemaValidation: true,
	})
	if err != nil {
		return nil, fmt.Errorf("load helm chart files: %w", err)
	}

	return helmChart, nil
}

// getCRDs gets CRD resources based on the chart configuration.
// It returns the list of CRD resources and any error encountered.
func (c *KCLPackage) getCRDs(
	chart *kclchart.ChartConfig,
	helmChart *helm.ChartFiles,
	logger *slog.Logger,
) ([]kube.Object, error) {
	switch chart.CRDGenerator {
	case crd.GeneratorTypeDefault, crd.GeneratorTypeAuto, crd.GeneratorTypeTemplate:
		logger.Info("rendering crd resources")

		crds, err := helmChart.GetCRDOutput()
		if err != nil {
			return nil, fmt.Errorf("render CRD resources: %w", err)
		}

		return crds, nil

	case crd.GeneratorTypeChartPath:
		logger.Info("getting crd files from chart")

		crds, err := helmChart.GetCRDFiles(crd.FromPaths, c.createCRDPathMatcher(chart))
		if err != nil {
			return nil, fmt.Errorf("get CRD files from chart: %w", err)
		}

		return crds, nil

	case crd.GeneratorTypePath:
		return c.getCRDsFromPaths(chart, logger)

	default: // Matches: crd.GeneratorTypeNone.
		logger.Debug("skipping crd generation")

		return []kube.Object{}, nil
	}
}

// getCRDsFromPaths gets CRDs from the chart's configured CRD paths.
func (c *KCLPackage) getCRDsFromPaths(
	chart *kclchart.ChartConfig,
	logger *slog.Logger,
) ([]kube.Object, error) {
	crds := []kube.Object{}
	for _, p := range chart.CRDPaths {
		pathCRDs, err := c.getCRDsFromPath(p, logger)
		if err != nil {
			return nil, err
		}

		crds = append(crds, pathCRDs...)
	}

	return crds, nil
}

// getCRDsFromPath gets CRDs from either a URL or a local file path.
func (c *KCLPackage) getCRDsFromPath(pathStr string, logger *slog.Logger) ([]kube.Object, error) {
	crdPath, err := paths.ResolveFilePathOrURL(c.BasePath, c.repoRoot, pathStr, []string{"http", "https"})
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", pathStr, err)
	}

	if u, ok := crdPath.URL(); ok {
		logger.Debug("getting crd files from url", slog.String("url", u.String()))

		crds, err := crd.FromURL(context.Background(), http.DefaultClient, u)
		if err != nil {
			return nil, fmt.Errorf("get CRDs from URL %q: %w", crdPath.String(), err)
		}

		return crds, nil
	}

	logger.Debug("getting crd files from local path", slog.String("path", pathStr))

	crds, err := crd.FromPath(crdPath.String())
	if err != nil {
		return nil, fmt.Errorf("get CRDs from file %q: %w", crdPath.String(), err)
	}

	return crds, nil
}

// createCRDPathMatcher creates a matcher function for CRD paths.
func (c *KCLPackage) createCRDPathMatcher(chart *kclchart.ChartConfig) func(string) bool {
	return func(s string) bool {
		for _, p := range chart.CRDPaths {
			match, err := filepath.Match(p, s)
			if err == nil && match {
				return true
			}
		}

		return false
	}
}

// updateChartsFile updates the charts.k file with the new chart.
func (c *KCLPackage) updateChartsFile(key string, chart *kclchart.ChartConfig, logger *slog.Logger) error {
	chartsFile := filepath.Join(c.BasePath, "charts.k")
	chartsSpec := kclautomation.SpecPathJoin("charts", key)

	logger.Info("updating charts.k",
		slog.String("spec", chartsSpec),
		slog.String("path", chartsFile),
	)

	err := c.updateFile(chart.ToAutomation(), chartsFile, initialChartContents, chartsSpec)
	if err != nil {
		return fmt.Errorf("update %q: %w", chartsFile, err)
	}

	return nil
}

// generateAndWriteChartKCL generates and writes the chart.k file.
func generateAndWriteChartKCL(hc *kclchart.Chart, chartDir string, logger *slog.Logger) error {
	kclChart := &bytes.Buffer{}

	logger.Debug("generating chart.k")

	err := hc.GenerateKCL(kclChart)
	if err != nil {
		return fmt.Errorf("generate chart.k: %w", err)
	}

	logger.Debug("writing chart.k")

	err = os.WriteFile(path.Join(chartDir, "chart.k"), kclChart.Bytes(), 0o600)
	if err != nil {
		return fmt.Errorf("write chart.k: %w", err)
	}

	return nil
}

// getValuesJSONSchema generates the JSON schema based on the chart configuration.
// It returns the schema bytes and any error encountered.
func (c *KCLPackage) getValuesJSONSchema(
	chart *kclchart.ChartConfig, chartFiles *helm.ChartFiles, logger *slog.Logger,
) ([]byte, error) {
	switch chart.SchemaGenerator {
	case schema.URLGeneratorType, schema.LocalPathGeneratorType:
		return c.generateSchemaFromPath(chart)

	case schema.AutoGeneratorType, schema.ValueInferenceGeneratorType, schema.ChartPathGeneratorType:
		return c.generateSchemaFromChart(chart, chartFiles)

	default: // Matches: schema.DefaultGeneratorType, schema.NoGeneratorType.
		logger.Info("no schema generator specified, values validation will be skipped")

		return []byte(schema.EmptySchema), nil
	}
}

// generateSchemaFromPath generates schema from a file path or URL.
func (c *KCLPackage) generateSchemaFromPath(chart *kclchart.ChartConfig) ([]byte, error) {
	schemaPath, err := paths.ResolveFilePathOrURL(c.pkgPath, c.repoRoot, chart.SchemaPath, []string{"http", "https"})
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", chart.SchemaPath, err)
	}

	jsonSchemaBytes, err := schema.DefaultReaderGenerator.FromPaths(schemaPath.String())
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", schemaPath.String(), err)
	}

	return jsonSchemaBytes, nil
}

// generateSchemaFromChart generates schema from chart files.
func (c *KCLPackage) generateSchemaFromChart(chart *kclchart.ChartConfig, chartFiles *helm.ChartFiles) ([]byte, error) {
	fileMatcher := schema.GetFileFilter(chart.SchemaGenerator)
	if chart.SchemaPath != "" {
		fileMatcher = func(f string) bool {
			return filePathsEqual(f, chart.SchemaPath)
		}
	}

	var gen helm.JSONSchemaGenerator

	switch chart.SchemaGenerator {
	case schema.AutoGeneratorType:
		gen = schema.DefaultAutoGenerator
	case schema.ValueInferenceGeneratorType:
		if chart.ValueInference == nil {
			gen = schema.DefaultValueInferenceGenerator
		} else {
			vig, err := schema.NewValueInferenceGenerator(chart.ValueInference.GetConfig())
			if err != nil {
				return nil, fmt.Errorf("create value inference generator: %w", err)
			}

			gen = vig
		}

	case schema.ChartPathGeneratorType:
		gen = schema.DefaultReaderGenerator
	default:
		gen = schema.DefaultNoGenerator
	}

	jsonSchemaBytes, err := chartFiles.GetValuesJSONSchema(gen, fileMatcher)
	if err != nil {
		return nil, fmt.Errorf("generate values schema: %w", err)
	}

	return jsonSchemaBytes, nil
}

// writeValuesSchemaFiles writes the values schema files.
func writeValuesSchemaFiles(jsonSchema []byte, chartDir string) error {
	err := os.WriteFile(path.Join(chartDir, "values.schema.json"), jsonSchema, 0o600)
	if err != nil {
		return fmt.Errorf("write values.schema.json: %w", err)
	}

	kclSchema, err := schema.ConvertToKCLSchema(jsonSchema, true)
	if err != nil {
		return fmt.Errorf("convert values schema to KCL: %w", err)
	}

	err = os.WriteFile(path.Join(chartDir, "values.schema.k"), kclSchema, 0o600)
	if err != nil {
		return fmt.Errorf("write values.schema.k: %w", err)
	}

	return nil
}

// writeCRDFiles writes the CRD files.
func (c *KCLPackage) writeCRDFiles(crds []kube.Object, chartDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	crdPkgPath := filepath.Join(chartDir, "api")
	err := crd.GenerateKCL(ctx, crds, crdPkgPath)
	if err != nil {
		return fmt.Errorf("generate CRDs at %q: %w", crdPkgPath, err)
	}

	return nil
}

// filePathsEqual checks if two file paths are equal after cleaning.
func filePathsEqual(f1, f2 string) bool {
	return filepath.Clean(f1) == filepath.Clean(f2)
}
