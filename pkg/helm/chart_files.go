package helm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/kube"
)

var (
	ErrNoMatcher    = errors.New("no matcher provided")
	ErrChartExtract = errors.New("error extracting chart")
)

type JSONSchemaGenerator interface {
	FromPaths(paths ...string) ([]byte, error)
}

type CRDGenerator interface {
	FromPaths(paths ...string) ([]*unstructured.Unstructured, error)
}

type ChartFiles struct {
	Client       ChartClient
	closer       io.Closer
	TemplateOpts *TemplateOpts
	pulledChart  *PulledChart
	path         string
}

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

// GetValuesJSONSchema pulls a Helm chart using the provided [TemplateOpts], and
// then uses the [JSONSchemaGenerator] to generate a JSON Schema using one or
// more files from the chart. The [match] function can be used to match a subset
// of the pulled files in the chart directory for JSON Schema generation.
func (c *ChartFiles) GetValuesJSONSchema(gen JSONSchemaGenerator, match func(string) bool) ([]byte, error) {
	if match == nil {
		return nil, ErrNoMatcher
	}

	matchedFiles := []string{}

	err := filepath.Walk(c.path,
		func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("walk helm chart directory: %w", err)
			}

			relPath, err := filepath.Rel(c.path, path)
			if err != nil {
				return fmt.Errorf("get relative path: %w", err)
			}
			// Use the relative path to match against the provided filter.
			if match(relPath) {
				// Append the unmodified/absolute path to the matched files.
				matchedFiles = append(matchedFiles, path)
			}

			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("read helm chart directory: %w", err)
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

func (c *ChartFiles) GetCRDOutput() ([]*unstructured.Unstructured, error) {
	loadedChart, err := c.pulledChart.Load(context.Background(), c.TemplateOpts.SkipSchemaValidation)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	out, err := templateData(context.Background(), loadedChart, c.TemplateOpts)
	if err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}

	resources, err := kube.SplitYAML(out)
	if err != nil {
		return nil, fmt.Errorf("split yaml: %w", err)
	}

	crdFiles := []*unstructured.Unstructured{}

	for _, r := range resources {
		if r.GetKind() == "CustomResourceDefinition" {
			crdFiles = append(crdFiles, r)
		}
	}

	return crdFiles, nil
}

func (c *ChartFiles) GetCRDFiles(gen CRDGenerator, match func(string) bool) ([]*unstructured.Unstructured, error) {
	if match == nil {
		return nil, ErrNoMatcher
	}

	matchedFiles := []string{}

	err := filepath.Walk(c.path,
		func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("walk helm chart directory: %w", err)
			}

			relPath, err := filepath.Rel(c.path, path)
			if err != nil {
				return fmt.Errorf("get relative path: %w", err)
			}
			// Use the relative path to match against the provided filter.
			if match(relPath) {
				// Append the unmodified/absolute path to the matched files.
				matchedFiles = append(matchedFiles, path)
			}

			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("read helm chart directory: %w", err)
	}

	crdFiles := []*unstructured.Unstructured{}

	if len(matchedFiles) == 0 {
		slog.Warn("no input files found for the CRD schema generator",
			slog.String("chart", c.TemplateOpts.ChartName),
			slog.String("path", c.path),
		)

		return crdFiles, nil
	}

	crdFiles, err = gen.FromPaths(matchedFiles...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CRDs from matched files: %w", err)
	}

	return crdFiles, nil
}

func (c *ChartFiles) Dispose() {
	if c.closer != nil {
		tryClose(c.closer)
	}
}
