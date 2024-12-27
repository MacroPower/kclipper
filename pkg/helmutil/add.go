package helmutil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"

	"github.com/iancoleman/strcase"
	"kcl-lang.io/kcl-go"
	"kcl-lang.io/kcl-go/pkg/tools/gen"
	kclutil "kcl-lang.io/kcl-go/pkg/utils"

	"github.com/MacroPower/kclx/pkg/helm"
	helmchart "github.com/MacroPower/kclx/pkg/helm/chart"
	"github.com/MacroPower/kclx/pkg/helm/schemagen"
	"github.com/MacroPower/kclx/pkg/util/safekcl"
)

var (
	SchemaDefaultRegexp = regexp.MustCompile(`(\s+\S+:\s+\S+(\s+\|\s+\S+)*)(\s+=\s+\S+)`)
	SchemaValuesRegexp  = regexp.MustCompile(`(\s+values\??\s*:\s+)(.*)`)
)

const initialMainContents = `import helm

charts: helm.Charts = {}
`

func (c *ChartPkg) Add(chart, repoURL, targetRevision string) error {
	return c.AddWithSchema(chart, repoURL, targetRevision, "", schemagen.AutoGenerator)
}

func (c *ChartPkg) AddWithSchema(
	chart, repoURL, targetRevision, schemaURL string,
	schemaGenerator schemagen.Generator,
) error {
	enableOCI, repoNetURL, err := c.parseRepoURL(repoURL)
	if err != nil {
		return err
	}

	chartSnake := strcase.ToSnake(chart)
	vendorDir := c.BasePath
	chartsDir := path.Join(vendorDir, chartSnake)

	if err := c.createChartsDir(chartsDir); err != nil {
		return err
	}

	if err := c.generateAndWriteChartK(chart, repoNetURL.String(), targetRevision, chartsDir); err != nil {
		return err
	}

	if schemaGenerator == schemagen.NoGenerator {
		return nil
	}

	jsBytes, err := c.fetchOrInferSchema(chart, targetRevision, repoNetURL.String(),
		enableOCI, schemaURL, schemaGenerator)
	if err != nil {
		return err
	}

	if err := c.writeSchemaFiles(chartsDir, jsBytes); err != nil {
		return err
	}

	chartConfig := []string{
		fmt.Sprintf(`chart="%s"`, chart),
		fmt.Sprintf(`repoURL="%s"`, repoNetURL.String()),
		fmt.Sprintf(`targetRevision="%s"`, targetRevision),
		fmt.Sprintf(`schemaGenerator="%s"`, schemaGenerator),
	}
	if err := c.updateMainFile(vendorDir, chartSnake, chartConfig...); err != nil {
		return err
	}

	_, err = kcl.FormatPath(vendorDir)
	if err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}

func (c *ChartPkg) parseRepoURL(repoURL string) (bool, *url.URL, error) {
	repoNetURL, err := url.Parse(repoURL)
	if err != nil {
		return false, nil, fmt.Errorf("failed to parse repo_url %s: %w", repoURL, err)
	}
	enableOCI := repoNetURL.Scheme == ""
	return enableOCI, repoNetURL, nil
}

func (c *ChartPkg) createChartsDir(chartsDir string) error {
	if err := os.MkdirAll(chartsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create charts directory: %w", err)
	}
	return nil
}

func (c *ChartPkg) generateAndWriteChartK(chart, repoURL, targetRevision, chartsDir string) error {
	kclChart := &bytes.Buffer{}
	hcc := helmchart.Chart{
		Chart:          chart,
		RepoURL:        repoURL,
		TargetRevision: targetRevision,
	}
	if err := hcc.GenerateKcl(kclChart); err != nil {
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

	if err := os.WriteFile(path.Join(chartsDir, "chart.k"), kclChartFixed.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write chart.k: %w", err)
	}
	return nil
}

func (c *ChartPkg) fetchOrInferSchema(chart, targetRevision, repoURL string, enableOCI bool, schemaURL string, schemaGenerator schemagen.Generator) ([]byte, error) {
	var jsBytes []byte
	var err error
	if schemaURL != "" {
		jsBytes, err = getSchemaFromURL(schemaURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch schema from %s: %w", schemaURL, err)
		}
	} else {
		jsBytes, err = helm.DefaultHelm.GetValuesJSONSchema(&helm.TemplateOpts{
			ChartName:       chart,
			TargetRevision:  targetRevision,
			RepoURL:         repoURL,
			EnableOCI:       enableOCI,
			PassCredentials: false,
		}, schemaGenerator == schemagen.AutoGenerator)
		if err != nil {
			return nil, fmt.Errorf("failed to infer values schema: %w", err)
		}
	}
	return jsBytes, nil
}

func (c *ChartPkg) writeSchemaFiles(chartsDir string, jsBytes []byte) error {
	if err := os.WriteFile(path.Join(chartsDir, "values.schema.json"), jsBytes, 0o600); err != nil {
		return fmt.Errorf("failed to write values.schema.json: %w", err)
	}

	kclSchema := &bytes.Buffer{}
	if err := safekcl.Gen.GenKcl(kclSchema, "values", jsBytes, &gen.GenKclOptions{
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

	if err := os.WriteFile(path.Join(chartsDir, "values.schema.k"), kclSchemaFixed.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write values.schema.k: %w", err)
	}
	return nil
}

func (c *ChartPkg) updateMainFile(vendorDir, chartSnake string, chartConfig ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	mainFile := path.Join(vendorDir, "main.k")
	if !kclutil.FileExists(mainFile) {
		if err := os.WriteFile(mainFile, []byte(initialMainContents), 0o600); err != nil {
			return fmt.Errorf("failed to write '%s': %w", mainFile, err)
		}
	}
	imports := []string{"helm"}
	specs := []string{}
	for _, cc := range chartConfig {
		specs = append(specs, fmt.Sprintf(`charts.%s.%s`, chartSnake, cc))
	}
	_, err := kcl.OverrideFile(mainFile, specs, imports)
	if err != nil {
		return fmt.Errorf("failed to update '%s': %w", mainFile, err)
	}
	return nil
}

func getSchemaFromURL(schemaURL string) ([]byte, error) {
	schemaNetURL, err := url.Parse(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	schema, err := http.DefaultClient.Do(&http.Request{
		Method: http.MethodGet,
		URL:    schemaNetURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed http request: %w", err)
	}

	jsBytes, err := io.ReadAll(schema.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	if err := schema.Body.Close(); err != nil {
		return nil, fmt.Errorf("failed to close body: %w", err)
	}

	return jsBytes, nil
}
