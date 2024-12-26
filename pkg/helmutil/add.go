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

	"kcl-lang.io/kcl-go"
	"kcl-lang.io/kcl-go/pkg/tools/gen"

	"github.com/MacroPower/kclx/pkg/helm"
	helmchart "github.com/MacroPower/kclx/pkg/helm/chart"
	genutil "github.com/MacroPower/kclx/pkg/util/gen"
)

var SchemaDefaultRegexp = regexp.MustCompile(`(\s+\S+:\s+\S+(\s+\|\s+\S+)*)(\s+=\s+\S+)`)

type SchemaMode int

const (
	SchemaAuto SchemaMode = iota
	SchemaFromValues
	SchemaNone
)

type ChartAdd struct {
	SchemaMode SchemaMode
	SchemaURL  string
	BasePath   string
}

func (ca *ChartAdd) Add(chart, repoURL, targetRevision string) error {
	enableOCI := false
	repoNetURL, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("failed to parse repo_url %s: %w", repoURL, err)
	}
	if repoNetURL.Scheme == "" {
		enableOCI = true
	}

	vendorDir := path.Join(ca.BasePath, chart)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		return fmt.Errorf("failed to create charts directory: %w", err)
	}

	// Add chart.k
	kclChart := &bytes.Buffer{}
	hcs := helmchart.Chart{
		Chart:          chart,
		RepoURL:        repoNetURL.String(),
		TargetRevision: targetRevision,
	}
	err = hcs.GenerateKcl(kclChart)
	if err != nil {
		return fmt.Errorf("failed to generate chart.k: %w", err)
	}

	kclChartFixed := &bytes.Buffer{}
	kclChartScanner := bufio.NewScanner(kclChart)
	for kclChartScanner.Scan() {
		line := kclChartScanner.Text()
		if line == "schema Chart:" {
			line = "import helm\n\nschema Chart(helm.Chart):"
		}
		kclChartFixed.WriteString(line + "\n")
	}
	if err := kclChartScanner.Err(); err != nil {
		return fmt.Errorf("failed to scan kcl schema: %w", err)
	}

	if err := os.WriteFile(path.Join(vendorDir, "chart.k"), kclChartFixed.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write chart.k: %w", err)
	}

	if ca.SchemaMode == SchemaNone && ca.SchemaURL == "" {
		return nil
	}

	var jsBytes []byte
	if ca.SchemaURL != "" {
		jsBytes, err = getSchemaFromURL(ca.SchemaURL)
		if err != nil {
			return fmt.Errorf("failed to fetch schema from %s: %w", ca.SchemaURL, err)
		}
	} else {
		jsBytes, err = helm.DefaultHelm.GetValuesJSONSchema(&helm.TemplateOpts{
			ChartName:       chart,
			TargetRevision:  targetRevision,
			RepoURL:         repoNetURL.String(),
			EnableOCI:       enableOCI,
			PassCredentials: false,
		}, ca.SchemaMode == SchemaAuto)
		if err != nil {
			return fmt.Errorf("failed to infer values schema: %w", err)
		}
	}

	if err := os.WriteFile(path.Join(vendorDir, "values.schema.json"), jsBytes, 0o600); err != nil {
		return fmt.Errorf("failed to write values.schema.json: %w", err)
	}

	kclSchema := &bytes.Buffer{}
	if err := genutil.Safe.GenKcl(kclSchema, "values", jsBytes, &gen.GenKclOptions{
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

	if err := os.WriteFile(path.Join(vendorDir, "values.schema.k"), kclSchemaFixed.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write values.schema.k: %w", err)
	}

	_, err = kcl.FormatPath(vendorDir)
	if err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
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
