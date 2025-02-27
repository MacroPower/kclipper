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
	if err := c.Init(); err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}

	absBasePath, err := filepath.Abs(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	chartDir := path.Join(absBasePath, key)
	if err := os.MkdirAll(chartDir, 0o750); err != nil {
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

	helmChart, err := helm.NewChartFiles(c.Client, repoMgr, &helm.TemplateOpts{
		ChartName:       chart.Chart,
		TargetRevision:  chart.TargetRevision,
		RepoURL:         chart.RepoURL,
		SkipCRDs:        chart.SkipCRDs,
		PassCredentials: chart.PassCredentials,
	})
	if err != nil {
		//nolint:wrapcheck // Error wrapped downstream.
		return err
	}
	defer helmChart.Dispose()

	logger := slog.With(
		slog.String("chart", chart.Chart),
	)

	if err := generateAndWriteChartKCL(&kclchart.Chart{ChartBase: chart.ChartBase}, chartDir, logger); err != nil {
		return err
	}

	if err := generateAndWriteValuesSchemaFiles(chart, helmChart, pkgPath, repoRoot, chartDir, logger); err != nil {
		return err
	}

	if chart.CRDPath != "" {
		logger.Debug("getting crd files")
		crds, err := helmChart.GetCRDs(func(s string) bool {
			match, err := filepath.Match(chart.CRDPath, s)
			if err != nil {
				return false
			}

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
	chartsSpec := kclutil.SpecPathJoin("charts", key)

	slog.Debug("updating charts.k")
	if err := c.updateFile(chart.ToAutomation(), chartsFile, initialChartContents, chartsSpec); err != nil {
		return fmt.Errorf("failed to update %q: %w", chartsFile, err)
	}

	slog.Debug("formatting kcl files", slog.String("path", c.BasePath))
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
