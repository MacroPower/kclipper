package helmutil

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
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

func (c *ChartPkg) AddChart(key string, chart *kclchart.ChartConfig) error {
	if err := chart.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	logger := slog.With(
		slog.String("cmd", "chart_add"),
		slog.String("chart_key", key),
		slog.String("chart", chart.Chart),
	)

	logger.Info("check init before add")
	if _, err := c.Init(); err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}

	absBasePath, err := filepath.Abs(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	chartDir := path.Join(absBasePath, key)
	logger.Debug("ensure chart directory", slog.String("path", chartDir))
	if err := os.MkdirAll(chartDir, 0o750); err != nil {
		return fmt.Errorf("failed to create charts directory: %w", err)
	}
	logger.Debug("ensured chart directory", slog.String("path", chartDir))

	logger.Debug("looking for repository root", slog.String("path", c.BasePath))
	repoRoot, err := kclutil.FindRepoRoot(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to find repository root: %w", err)
	}
	logger.Debug("found repository root", slog.String("path", repoRoot))

	logger.Debug("looking for topmost kcl.mod file",
		slog.String("begin", absBasePath),
		slog.String("end", repoRoot),
	)
	pkgPath, err := kclutil.FindTopPkgRoot(repoRoot, c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to find package root: %w", err)
	}
	logger.Debug("found topmost kcl.mod file", slog.String("path", pkgPath))

	logger.Info("loading helm repositories")
	repoMgr := helmrepo.NewManager(helmrepo.WithAllowedPaths(pkgPath, repoRoot))
	for _, repo := range chart.Repositories {
		logger.Debug("adding helm repository",
			slog.String("name", repo.Name),
			slog.String("url", repo.URL),
		)

		hr, err := repo.GetHelmRepo()
		if err != nil {
			return fmt.Errorf("failed to add Helm repository: %w", err)
		}

		if err := repoMgr.Add(hr); err != nil {
			return fmt.Errorf("failed to add Helm repository: %w", err)
		}
	}

	chartValues := map[string]any{}
	if chart.Values != nil {
		var ok bool
		chartValues, ok = chart.Values.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid values type: %T", chart.Values)
		}
	}

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
		//nolint:wrapcheck // Error wrapped downstream.
		return err
	}
	defer helmChart.Dispose()

	if err := generateAndWriteChartKCL(&kclchart.Chart{ChartBase: chart.ChartBase}, chartDir, logger); err != nil {
		return err
	}

	if err := generateAndWriteValuesSchemaFiles(chart, helmChart, pkgPath, repoRoot, chartDir, logger); err != nil {
		return err
	}

	crds := [][]byte{}
	switch chart.CRDGenerator {
	case kclutil.CRDGeneratorTypeDefault, kclutil.CRDGeneratorTypeTemplate:
		logger.Info("rendering crd resources")
		crdResources, err := helmChart.GetCRDOutput()
		if err != nil {
			return fmt.Errorf("failed to render CRD resources: %w", err)
		}
		crds = append(crds, crdResources...)
	case kclutil.CRDGeneratorTypeChartPath:
		logger.Info("getting crd files")
		crdFiles, err := helmChart.GetCRDFiles(func(s string) bool {
			match, err := filepath.Match(chart.CRDPath, s)
			if err != nil {
				return false
			}

			return match
		})
		if err != nil {
			return fmt.Errorf("failed to get CRD files: %w", err)
		}
		crds = append(crds, crdFiles...)
	case kclutil.CRDGeneratorTypeNone:
		logger.Debug("skipping crd generation")
	}
	if len(crds) > 0 {
		if err := c.writeCRDFiles(crds, chartDir); err != nil {
			return err
		}
	}

	chartsFile := filepath.Join(c.BasePath, "charts.k")
	chartsSpec := kclutil.SpecPathJoin("charts", key)

	logger.Info("updating charts.k",
		slog.String("spec", chartsSpec),
		slog.String("path", chartsFile),
	)
	if err := c.updateFile(chart.ToAutomation(), chartsFile, initialChartContents, chartsSpec); err != nil {
		return fmt.Errorf("failed to update %q: %w", chartsFile, err)
	}

	logger.Info("formatting kcl files", slog.String("path", c.BasePath))
	if _, err := kcl.FormatPath(c.BasePath); err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}

func generateAndWriteChartKCL(hc *kclchart.Chart, chartDir string, logger *slog.Logger) error {
	kclChart := &bytes.Buffer{}

	logger.Debug("generating chart.k")
	if err := hc.GenerateKCL(kclChart); err != nil {
		return fmt.Errorf("failed to generate chart.k: %w", err)
	}

	logger.Debug("writing chart.k")
	if err := os.WriteFile(path.Join(chartDir, "chart.k"), kclChart.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write chart.k: %w", err)
	}

	return nil
}

func generateAndWriteValuesSchemaFiles(
	chart *kclchart.ChartConfig, chartFiles *helm.ChartFiles, basePath, repoRoot, chartDir string, logger *slog.Logger,
) error {
	var (
		jsonSchemaBytes []byte
		err             error
	)

	logger.Debug("generating values.schema.json")
	switch chart.SchemaGenerator {
	case jsonschema.DefaultGeneratorType, jsonschema.NoGeneratorType:
		logger.Info("no schema generator specified, values validation will be skipped")
		jsonSchemaBytes = []byte(jsonschema.EmptySchema)

	case jsonschema.URLGeneratorType, jsonschema.LocalPathGeneratorType:
		schemaPath, err := pathutil.ResolveFilePathOrURL(basePath, repoRoot, chart.SchemaPath, []string{"http", "https"})
		if err != nil {
			return fmt.Errorf("failed to resolve schema path: %w", err)
		}

		jsonSchemaBytes, err = jsonschema.DefaultReaderGenerator.FromPaths(schemaPath.String())
		if err != nil {
			return fmt.Errorf("failed to fetch schema from %q: %w", schemaPath.String(), err)
		}

	case jsonschema.AutoGeneratorType,
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
		logger.Debug("writing values.schema.json")
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

func (c *ChartPkg) writeCRDFiles(crds [][]byte, chartDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	workerCount := int64(runtime.GOMAXPROCS(0))
	crdCount := len(crds)
	sem := semaphore.NewWeighted(workerCount)
	errChan := make(chan error, crdCount)

	for _, crd := range crds {
		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to acquire worker: %w", err)
		}
		go func() {
			defer sem.Release(1)

			errChan <- kclutil.GenOpenAPI.FromCRD(crd, chartDir)
		}()
	}

	if err := sem.Acquire(ctx, workerCount); err != nil {
		return fmt.Errorf("failed to generate KCL from CRD: %w", err)
	}

	close(errChan)
	var merr error
	for err := range errChan {
		if err != nil {
			merr = multierror.Append(merr, err)
		}
	}
	if merr != nil {
		return fmt.Errorf("failed to generate KCL from CRD: %w", merr)
	}

	return nil
}

func filePathsEqual(f1, f2 string) bool {
	return filepath.Clean(f1) == filepath.Clean(f2)
}
