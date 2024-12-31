package helmutil

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"kcl-lang.io/kcl-go"
	"kcl-lang.io/kcl-go/pkg/tools/gen"

	"github.com/MacroPower/kclx/pkg/helm"
	"github.com/MacroPower/kclx/pkg/helmmodels"
	"github.com/MacroPower/kclx/pkg/jsonschema"
	"github.com/MacroPower/kclx/pkg/kclutil"
)

var (
	SchemaDefaultRegexp = regexp.MustCompile(`(\s+\S+:\s+\S+(\s+\|\s+\S+)*)(\s+=\s+\S+)`)
	SchemaValuesRegexp  = regexp.MustCompile(`(\s+values\??\s*:\s+)(.*)`)
)

const initialMainContents = `import helm

charts: helm.Charts = {}
`

func (c *ChartPkg) Add(chart, repoURL, targetRevision, schemaPath string, genType jsonschema.GeneratorType) error {
	hc := helmmodels.Chart{
		ChartBase: helmmodels.ChartBase{
			Chart:          chart,
			RepoURL:        repoURL,
			TargetRevision: targetRevision,
		},
	}

	chartDir := path.Join(c.BasePath, hc.GetSnakeCaseName())

	if err := c.Init(); err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		return fmt.Errorf("failed to create charts directory: %w", err)
	}

	if err := c.generateAndWriteChartKCL(hc, chartDir); err != nil {
		return err
	}

	var jsonSchemaBytes []byte
	var err error

	switch genType {
	case jsonschema.NoGeneratorType:
		break
	case jsonschema.URLGeneratorType, jsonschema.LocalPathGeneratorType:
		jsonSchemaBytes, err = jsonschema.DefaultReaderGenerator.FromPaths(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to fetch schema from %s: %w", schemaPath, err)
		}
	case jsonschema.AutoGeneratorType, jsonschema.ValueInferenceGeneratorType, jsonschema.PathGeneratorType:
		fileMatcher := jsonschema.GetFileFilter(genType)
		if schemaPath != "" {
			fileMatcher = func(f string) bool {
				return filePathsEqual(f, schemaPath)
			}
		}
		helmChart := helm.NewChart(helm.DefaultClient, helm.TemplateOpts{
			ChartName:      chart,
			TargetRevision: targetRevision,
			RepoURL:        repoURL,
		})
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

	chartConfig := map[string]string{
		"chart":           chart,
		"repoURL":         repoURL,
		"targetRevision":  targetRevision,
		"schemaGenerator": string(genType),
		"schemaPath":      schemaPath,
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

	kclChartFixed := &bytes.Buffer{}
	kclChartScanner := bufio.NewScanner(kclChart)
	for kclChartScanner.Scan() {
		line := kclChartScanner.Text()
		if line == "schema Chart:" {
			line = "import helm\n\nschema Chart(helm.Chart):"
		} else if SchemaValuesRegexp.MatchString(line) {
			line = SchemaValuesRegexp.ReplaceAllString(line, "${1}Values | ${2}")
		}
		kclChartFixed.WriteString(line + "\n")
	}
	if err := kclChartScanner.Err(); err != nil {
		return fmt.Errorf("failed to scan kcl schema: %w", err)
	}

	if err := os.WriteFile(path.Join(chartDir, "chart.k"), kclChartFixed.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write chart.k: %w", err)
	}
	return nil
}

func (c *ChartPkg) writeValuesSchemaFiles(jsonSchema []byte, chartDir string) error {
	if err := os.WriteFile(path.Join(chartDir, "values.schema.json"), jsonSchema, 0o600); err != nil {
		return fmt.Errorf("failed to write values.schema.json: %w", err)
	}

	kclSchema := &bytes.Buffer{}
	if err := kclutil.Gen.GenKcl(kclSchema, "values", jsonSchema, &gen.GenKclOptions{
		Mode:                  gen.ModeJsonSchema,
		CastingOption:         gen.OriginalName,
		UseIntegersForNumbers: true,
	}); err != nil {
		return fmt.Errorf("failed to generate kcl schema: %w", err)
	}

	kclSchemaFixed := &bytes.Buffer{}
	scanner := bufio.NewScanner(kclSchema)
	for scanner.Scan() {
		line := scanner.Text()
		line = SchemaDefaultRegexp.ReplaceAllString(line, "$1")
		kclSchemaFixed.WriteString(line + "\n")
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan kcl schema: %w", err)
	}

	if err := os.WriteFile(path.Join(chartDir, "values.schema.k"), kclSchemaFixed.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write values.schema.k: %w", err)
	}
	return nil
}

func (c *ChartPkg) updateChartsFile(vendorDir, chartKey string, chartConfig map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	mainFile := path.Join(vendorDir, "charts.k")
	if !fileExists(mainFile) {
		if err := os.WriteFile(mainFile, []byte(initialMainContents), 0o600); err != nil {
			return fmt.Errorf("failed to write '%s': %w", mainFile, err)
		}
	}
	imports := []string{"helm"}
	specs := []string{}
	for k, v := range chartConfig {
		if k == "" {
			return fmt.Errorf("invalid key in chart config: %#v", chartConfig)
		}
		if v == "" {
			continue
		}
		specs = append(specs, fmt.Sprintf(`charts.%s.%s="%s"`, chartKey, k, v))
	}
	_, err := kcl.OverrideFile(mainFile, specs, imports)
	if err != nil {
		return fmt.Errorf("failed to update '%s': %w", mainFile, err)
	}
	return nil
}

func filePathsEqual(f1, f2 string) bool {
	return filepath.Clean(f1) == filepath.Clean(f2)
}
