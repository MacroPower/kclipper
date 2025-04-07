package helmutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/crd"
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

// Common error variables for chart operations.
var (
	ErrInvalidConfig     = errors.New("invalid chart configuration")
	ErrInitFailed        = errors.New("failed to initialize")
	ErrPathResolution    = errors.New("path resolution failed")
	ErrDirectoryCreation = errors.New("directory creation failed")
	ErrRepoOperation     = errors.New("repository operation failed")
	ErrChartOperation    = errors.New("chart operation failed")
	ErrSchemaGeneration  = errors.New("schema generation failed")
	ErrCRDGeneration     = errors.New("CRD generation failed")
	ErrFileWrite         = errors.New("file write failed")
	ErrKCLOperation      = errors.New("KCL operation failed")
)

// AddChart adds a new chart to the chart package.
func (c *ChartPkg) AddChart(key string, chart *kclchart.ChartConfig) error {
	if err := chart.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	logger := slog.With(
		slog.String("cmd", "chart_add"),
		slog.String("chart_key", key),
		slog.String("chart", chart.Chart),
	)

	logger.Info("check init before add")
	if _, err := c.Init(); err != nil {
		return fmt.Errorf("%w: %w", ErrInitFailed, err)
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

	if err := generateAndWriteChartKCL(&kclchart.Chart{ChartBase: chart.ChartBase}, chartDir, logger); err != nil {
		return err
	}

	jsonSchemaBytes, err := c.getValuesJSONSchema(chart, helmChart, logger)
	if err != nil {
		return err
	}
	if len(jsonSchemaBytes) != 0 {
		if err := writeValuesSchemaFiles(jsonSchemaBytes, chartDir); err != nil {
			return err
		}
	}

	crds, err := c.getCRDs(chart, helmChart, logger)
	if err != nil {
		return err
	}
	if len(crds) > 0 {
		if err := c.writeCRDFiles(crds, chartDir); err != nil {
			return err
		}
	}

	if err := c.updateChartsFile(key, chart, logger); err != nil {
		return err
	}

	logger.Info("formatting kcl files", slog.String("path", chartDir))
	if _, err := kcl.FormatPath(filepath.Join(chartDir, "...")); err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}

// addChartDir prepares the necessary directories for the chart.
func (c *ChartPkg) addChartDir(key string, logger *slog.Logger) (string, error) {
	chartDir := path.Join(c.absBasePath, key)
	logger.Debug("ensure chart directory", slog.String("path", chartDir))
	if err := os.MkdirAll(chartDir, 0o750); err != nil {
		return "", fmt.Errorf("%w: create charts directory: %w", ErrDirectoryCreation, err)
	}
	logger.Debug("ensured chart directory", slog.String("path", chartDir))

	return chartDir, nil
}

// setupHelmChart sets up the helm repositories and creates a chart.
func (c *ChartPkg) setupHelmChart(chart *kclchart.ChartConfig, logger *slog.Logger) (*helm.ChartFiles, error) {
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
			return nil, fmt.Errorf("%w: get Helm repository: %w", ErrRepoOperation, err)
		}

		if err := repoMgr.Add(hr); err != nil {
			return nil, fmt.Errorf("%w: add Helm repository: %w", ErrRepoOperation, err)
		}
	}

	// Prepare chart values.
	chartValues := map[string]any{}
	if chart.Values != nil {
		var ok bool
		chartValues, ok = chart.Values.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: invalid values type: %T", ErrInvalidConfig, chart.Values)
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
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartOperation, err)
	}

	return helmChart, nil
}

// getCRDs gets CRD resources based on the chart configuration.
// It returns the list of CRD resources and any error encountered.
func (c *ChartPkg) getCRDs(chart *kclchart.ChartConfig, helmChart *helm.ChartFiles, logger *slog.Logger) ([]*unstructured.Unstructured, error) {
	switch chart.CRDGenerator {
	case crd.GeneratorTypeDefault, crd.GeneratorTypeAuto, crd.GeneratorTypeTemplate:
		logger.Info("rendering crd resources")

		return c.getCRDsFromTemplate(helmChart)

	case crd.GeneratorTypeChartPath:
		logger.Info("getting crd files from chart")

		return c.getCRDsFromChartPath(chart, helmChart)

	case crd.GeneratorTypePath:
		return c.getCRDsFromPaths(chart, logger)

	default: // Matches: crd.GeneratorTypeNone.
		logger.Debug("skipping crd generation")

		return []*unstructured.Unstructured{}, nil
	}
}

// getCRDsFromTemplate generates CRD resources from helm template output.
func (c *ChartPkg) getCRDsFromTemplate(helmChart *helm.ChartFiles) ([]*unstructured.Unstructured, error) {
	crds, err := helmChart.GetCRDOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: render CRD resources: %w", ErrCRDGeneration, err)
	}

	return crds, nil
}

// getCRDsFromChartPath generates CRD resources from the chart path.
func (c *ChartPkg) getCRDsFromChartPath(chart *kclchart.ChartConfig, helmChart *helm.ChartFiles) ([]*unstructured.Unstructured, error) {
	crds, err := helmChart.GetCRDFiles(crd.DefaultFileGenerator, c.createCRDPathMatcher(chart))
	if err != nil {
		return nil, fmt.Errorf("%w: get CRD files from chart: %w", ErrCRDGeneration, err)
	}

	return crds, nil
}

// getCRDsFromPaths gets CRDs from specified paths.
func (c *ChartPkg) getCRDsFromPaths(chart *kclchart.ChartConfig, logger *slog.Logger) ([]*unstructured.Unstructured, error) {
	crds := []*unstructured.Unstructured{}
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
func (c *ChartPkg) getCRDsFromPath(pathStr string, logger *slog.Logger) ([]*unstructured.Unstructured, error) {
	crdPath, err := pathutil.ResolveFilePathOrURL(c.BasePath, c.repoRoot, pathStr, []string{"http", "https"})
	if err != nil {
		return nil, fmt.Errorf("%w: resolve %q: %w", ErrPathResolution, pathStr, err)
	}
	if u, ok := crdPath.URL(); ok {
		logger.Debug("getting crd files from url", slog.String("url", u.String()))

		return c.getCRDsFromURL(u, crdPath.String())
	}
	logger.Debug("getting crd files from local path", slog.String("path", pathStr))

	return c.getCRDsFromFilePath(crdPath.String())
}

// getCRDsFromURL gets CRDs from a URL.
func (c *ChartPkg) getCRDsFromURL(u *url.URL, pathStr string) ([]*unstructured.Unstructured, error) {
	crdResources, err := crd.DefaultHTTPGenerator.FromURL(context.Background(), u)
	if err != nil {
		return nil, fmt.Errorf("%w: get CRDs from URL %q: %w", ErrCRDGeneration, pathStr, err)
	}

	return crdResources, nil
}

// getCRDsFromFilePath gets CRDs from a local file path.
func (c *ChartPkg) getCRDsFromFilePath(pathStr string) ([]*unstructured.Unstructured, error) {
	crdResources, err := crd.DefaultFileGenerator.FromPath(pathStr)
	if err != nil {
		return nil, fmt.Errorf("%w: get CRDs from file %q: %w", ErrCRDGeneration, pathStr, err)
	}

	return crdResources, nil
}

// createCRDPathMatcher creates a matcher function for CRD paths.
func (c *ChartPkg) createCRDPathMatcher(chart *kclchart.ChartConfig) func(string) bool {
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
func (c *ChartPkg) updateChartsFile(key string, chart *kclchart.ChartConfig, logger *slog.Logger) error {
	chartsFile := filepath.Join(c.BasePath, "charts.k")
	chartsSpec := kclutil.SpecPathJoin("charts", key)

	logger.Info("updating charts.k",
		slog.String("spec", chartsSpec),
		slog.String("path", chartsFile),
	)
	if err := c.updateFile(chart.ToAutomation(), chartsFile, initialChartContents, chartsSpec); err != nil {
		return fmt.Errorf("%w: update %q: %w", ErrFileWrite, chartsFile, err)
	}

	return nil
}

// generateAndWriteChartKCL generates and writes the chart.k file.
func generateAndWriteChartKCL(hc *kclchart.Chart, chartDir string, logger *slog.Logger) error {
	kclChart := &bytes.Buffer{}

	logger.Debug("generating chart.k")
	if err := hc.GenerateKCL(kclChart); err != nil {
		return fmt.Errorf("%w: generate chart.k: %w", ErrFileWrite, err)
	}

	logger.Debug("writing chart.k")
	if err := os.WriteFile(path.Join(chartDir, "chart.k"), kclChart.Bytes(), 0o600); err != nil {
		return fmt.Errorf("%w: write chart.k: %w", ErrFileWrite, err)
	}

	return nil
}

// getValuesJSONSchema generates the JSON schema based on the chart configuration.
// It returns the schema bytes and any error encountered.
func (c *ChartPkg) getValuesJSONSchema(
	chart *kclchart.ChartConfig, chartFiles *helm.ChartFiles, logger *slog.Logger,
) ([]byte, error) {
	switch chart.SchemaGenerator {
	case jsonschema.URLGeneratorType, jsonschema.LocalPathGeneratorType:
		return c.generateSchemaFromPath(chart)

	case jsonschema.AutoGeneratorType, jsonschema.ValueInferenceGeneratorType, jsonschema.ChartPathGeneratorType:
		return c.generateSchemaFromChart(chart, chartFiles)

	default: // Matches: jsonschema.DefaultGeneratorType, jsonschema.NoneGeneratorType.
		logger.Info("no schema generator specified, values validation will be skipped")

		return c.generateEmptySchema()
	}
}

// generateEmptySchema returns an empty schema for cases where no schema generation is needed.
func (c *ChartPkg) generateEmptySchema() ([]byte, error) {
	return []byte(jsonschema.EmptySchema), nil
}

// generateSchemaFromPath generates schema from a file path or URL.
func (c *ChartPkg) generateSchemaFromPath(chart *kclchart.ChartConfig) ([]byte, error) {
	schemaPath, err := pathutil.ResolveFilePathOrURL(c.pkgPath, c.repoRoot, chart.SchemaPath, []string{"http", "https"})
	if err != nil {
		return nil, fmt.Errorf("%w: resolve %q: %w", ErrPathResolution, chart.SchemaPath, err)
	}

	jsonSchemaBytes, err := jsonschema.DefaultReaderGenerator.FromPaths(schemaPath.String())
	if err != nil {
		return nil, fmt.Errorf("%w: get %q: %w", ErrSchemaGeneration, schemaPath.String(), err)
	}

	return jsonSchemaBytes, nil
}

// generateSchemaFromChart generates schema from chart files.
func (c *ChartPkg) generateSchemaFromChart(chart *kclchart.ChartConfig, chartFiles *helm.ChartFiles) ([]byte, error) {
	fileMatcher := jsonschema.GetFileFilter(chart.SchemaGenerator)
	if chart.SchemaPath != "" {
		fileMatcher = func(f string) bool {
			return filePathsEqual(f, chart.SchemaPath)
		}
	}

	var gen helm.JSONSchemaGenerator
	switch chart.SchemaGenerator {
	case jsonschema.AutoGeneratorType:
		gen = jsonschema.DefaultAutoGenerator
	case jsonschema.ValueInferenceGeneratorType:
		if chart.ValueInference == nil {
			gen = jsonschema.DefaultValueInferenceGenerator
		} else {
			gen = jsonschema.NewValueInferenceGenerator(chart.ValueInference.GetConfig())
		}
	case jsonschema.ChartPathGeneratorType:
		gen = jsonschema.DefaultReaderGenerator
	default:
		gen = jsonschema.DefaultNoGenerator
	}

	jsonSchemaBytes, err := chartFiles.GetValuesJSONSchema(gen, fileMatcher)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchemaGeneration, err)
	}

	return jsonSchemaBytes, nil
}

// writeValuesSchemaFiles writes the values schema files.
func writeValuesSchemaFiles(jsonSchema []byte, chartDir string) error {
	if err := os.WriteFile(path.Join(chartDir, "values.schema.json"), jsonSchema, 0o600); err != nil {
		return fmt.Errorf("%w: values.schema.json: %w", ErrFileWrite, err)
	}

	kclSchema, err := jsonschema.ConvertToKCLSchema(jsonSchema, true)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSchemaGeneration, err)
	}

	if err := os.WriteFile(path.Join(chartDir, "values.schema.k"), kclSchema, 0o600); err != nil {
		return fmt.Errorf("%w: values.schema.k: %w", ErrFileWrite, err)
	}

	return nil
}

// writeCRDFiles writes the CRD files.
func (c *ChartPkg) writeCRDFiles(crds []*unstructured.Unstructured, chartDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	crdPkgPath := filepath.Join(chartDir, "api")
	crdPkg := crd.NewKCLPackage(crds, crdPkgPath)
	err := crdPkg.GenerateC(ctx)
	if err != nil {
		return fmt.Errorf("%w: generate %q: %w", ErrCRDGeneration, crdPkgPath, err)
	}

	return nil
}

// filePathsEqual checks if two file paths are equal after cleaning.
func filePathsEqual(f1, f2 string) bool {
	return filepath.Clean(f1) == filepath.Clean(f2)
}
