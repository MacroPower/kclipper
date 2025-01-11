package helmutil

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"

	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/helm"
	helmmodels "github.com/MacroPower/kclipper/pkg/helmmodels/chartmodule"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

var (
	SchemaInvalidDocRegexp = regexp.MustCompile(`(\s+\S.*)r"""(.*)"""(.*)`)
	SchemaDefaultRegexp    = regexp.MustCompile(`(\s+\S+:\s+\S+(\s+\|\s+\S+)*)(\s+=.+)`)
)

const initialMainContents = `import helm

charts: helm.Charts = {}
`

func (c *ChartPkg) Add(
	chart, repoURL, targetRevision, schemaPath, crdPath string,
	genType jsonschema.GeneratorType,
	validateType jsonschema.ValidatorType,
) error {
	hc := helmmodels.Chart{
		ChartBase: helmmodels.ChartBase{
			Chart:           chart,
			RepoURL:         repoURL,
			TargetRevision:  targetRevision,
			SchemaValidator: validateType,
		},
	}

	chartDir := path.Join(c.BasePath, hc.GetSnakeCaseName())

	if err := c.Init(); err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		return fmt.Errorf("failed to create charts directory: %w", err)
	}

	helmChart, err := helm.NewChartFiles(c.Client, helm.TemplateOpts{
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

	kclSchema, err := jsonschema.ConvertToKCLSchema(jsonSchema)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	kclSchemaFixed := &bytes.Buffer{}
	scanner := bufio.NewScanner(bytes.NewReader(kclSchema))
	for scanner.Scan() {
		line := scanner.Text()
		line = SchemaDefaultRegexp.ReplaceAllString(line, "$1")
		line = SchemaInvalidDocRegexp.ReplaceAllString(line, `${1}"${2}"${3}`)
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

func (c *ChartPkg) writeCRDFiles(crds [][]byte, chartDir string) error {
	f, err := os.Create(path.Join(chartDir, "crds.schema.k"))
	if err != nil {
		return fmt.Errorf("failed to create crd.k: %w", err)
	}
	defer f.Close()

	tmpDir := os.TempDir()

	for _, crd := range crds {
		// Generator only accepts files, so just doing this for now to avoid
		// importing and re-writing a bunch of code.
		f, err := os.CreateTemp(tmpDir, "helm-crd-")
		if err != nil {
			return fmt.Errorf("failed to create temp file for CRDs: %w", err)
		}
		_, err = f.Write(crd)
		if err != nil {
			return fmt.Errorf("failed to write CRD to temp file: %w", err)
		}
		err = f.Close()
		if err != nil {
			return fmt.Errorf("failed to close temp file: %w", err)
		}
		defer func() {
			_ = os.Remove(f.Name())
		}()

		err = CRDToKCL(f.Name(), chartDir)
		if err != nil {
			return fmt.Errorf("failed to generate KCL from CRD: %w", err)
		}
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
	_, err := kcl.OverrideFile(mainFile, specs, imports)
	if err != nil {
		return fmt.Errorf("failed to update '%s': %w", mainFile, err)
	}
	return nil
}

func filePathsEqual(f1, f2 string) bool {
	return filepath.Clean(f1) == filepath.Clean(f2)
}
