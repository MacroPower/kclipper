package helm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/macropower/kclipper/pkg/helmrepo"
	"github.com/macropower/kclipper/pkg/kube"
)

var (
	// ErrNoMatcher indicates that no matcher function was provided.
	ErrNoMatcher = errors.New("no matcher provided")

	// ErrChartExtract indicates an error occurred while extracting a chart.
	ErrChartExtract = errors.New("extract chart")
)

// JSONSchemaGenerator generates JSON Schema from one or more file paths.
// See [jsonschema.AutoGenerator] for an implementation.
type JSONSchemaGenerator interface {
	FromPaths(paths ...string) ([]byte, error)
}

// CRDGenerator generates [kube.Object] slices from one or more file paths.
// See [crd.FromPaths] for an implementation.
type CRDGenerator func(paths ...string) ([]kube.Object, error)

// ChartFiles provides access to files within a pulled and extracted Helm chart.
// Create instances with [NewChartFiles].
type ChartFiles struct {
	Client       ChartClient
	closer       io.Closer
	TemplateOpts *TemplateOpts
	pulledChart  *PulledChart
	path         string
}

// NewChartFiles creates a new [ChartFiles].
func NewChartFiles(
	client ChartClient,
	repos helmrepo.Getter,
	maxSize *resource.Quantity,
	opts *TemplateOpts,
) (*ChartFiles, error) {
	cancel := func() {}
	ctx := context.Background()
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
	}

	defer cancel()

	pulledChart, err := client.Pull(ctx, opts.ChartName, opts.RepoURL, opts.TargetRevision, repos)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartPull, err)
	}

	chartPath, closer, err := pulledChart.Extract(maxSize)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartExtract, err)
	}

	return &ChartFiles{
		Client:       client,
		TemplateOpts: opts,
		path:         chartPath,
		pulledChart:  pulledChart,
		closer:       closer,
	}, nil
}

// matchChartFiles walks basePath and returns absolute paths of files whose
// path relative to basePath satisfies match.
func matchChartFiles(basePath string, match func(string) bool) ([]string, error) {
	var matchedFiles []string

	err := filepath.WalkDir(basePath, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return fmt.Errorf("get relative path: %w", err)
		}

		if match(relPath) {
			matchedFiles = append(matchedFiles, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read helm chart directory: %w", err)
	}

	return matchedFiles, nil
}

// GetValuesJSONSchema pulls a Helm chart using the provided [TemplateOpts], and
// then uses the [JSONSchemaGenerator] to generate a JSON Schema using one or
// more files from the chart. The [match] function can be used to match a subset
// of the pulled files in the chart directory for JSON Schema generation.
func (c *ChartFiles) GetValuesJSONSchema(gen JSONSchemaGenerator, match func(string) bool) ([]byte, error) {
	if match == nil {
		return nil, ErrNoMatcher
	}

	matchedFiles, err := matchChartFiles(c.path, match)
	if err != nil {
		return nil, fmt.Errorf("match values json schema files: %w", err)
	}

	if len(matchedFiles) == 0 {
		slog.Warn("no input files found for the provided JSON Schema generator",
			slog.String("chart", c.TemplateOpts.ChartName),
			slog.String("path", c.path),
		)

		return []byte{}, nil
	}

	jsonSchema, err := gen.FromPaths(matchedFiles...)
	if err != nil {
		return nil, fmt.Errorf("convert values to json schema: %w", err)
	}

	return jsonSchema, nil
}

// GetCRDOutput templates the chart and returns only the CRD resources.
func (c *ChartFiles) GetCRDOutput() ([]kube.Object, error) {
	loadedChart, err := c.pulledChart.Load(context.Background(), c.TemplateOpts.SkipSchemaValidation)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartLoad, err)
	}

	out, err := templateData(context.Background(), loadedChart, c.TemplateOpts)
	if err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}

	resources, err := kube.SplitYAML(out)
	if err != nil {
		return nil, fmt.Errorf("split yaml: %w", err)
	}

	crdFiles := []kube.Object{}

	for _, r := range resources {
		if r.IsCRD() {
			crdFiles = append(crdFiles, r)
		}
	}

	return crdFiles, nil
}

// GetCRDFiles returns CRD objects generated from chart files matching the
// provided function using the given [CRDGenerator].
func (c *ChartFiles) GetCRDFiles(gen CRDGenerator, match func(string) bool) ([]kube.Object, error) {
	if match == nil {
		return nil, ErrNoMatcher
	}

	matchedFiles, err := matchChartFiles(c.path, match)
	if err != nil {
		return nil, fmt.Errorf("match crd files: %w", err)
	}

	crdFiles := []kube.Object{}

	if len(matchedFiles) == 0 {
		slog.Warn("no input files found for the CRD schema generator",
			slog.String("chart", c.TemplateOpts.ChartName),
			slog.String("path", c.path),
		)

		return crdFiles, nil
	}

	crdFiles, err = gen(matchedFiles...)
	if err != nil {
		return nil, fmt.Errorf("fetch CRDs from matched files: %w", err)
	}

	return crdFiles, nil
}

// Dispose releases the resources associated with the extracted chart.
func (c *ChartFiles) Dispose() {
	if c.closer != nil {
		tryClose(c.closer)
	}
}
