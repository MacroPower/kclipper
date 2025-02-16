package helm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/MacroPower/kclipper/pkg/helmrepo"
)

var ErrNoMatcher = errors.New("no matcher provided")

type JSONSchemaGenerator interface {
	FromPaths(paths ...string) ([]byte, error)
}

type ChartFileClient interface {
	PullAndExtract(ctx context.Context, chartName, repoURL, targetRevision string, repos helmrepo.Getter) (string, io.Closer, error)
}

type ChartFiles struct {
	Client       ChartFileClient
	closer       io.Closer
	TemplateOpts *TemplateOpts
	path         string
}

func NewChartFiles(client ChartFileClient, repos helmrepo.Getter, opts *TemplateOpts) (*ChartFiles, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	chartPath, closer, err := client.PullAndExtract(ctx, opts.ChartName, opts.RepoURL, opts.TargetRevision, repos)
	if err != nil {
		return nil, fmt.Errorf("error pulling helm chart: %w", err)
	}

	return &ChartFiles{
		Client:       client,
		TemplateOpts: opts,
		path:         chartPath,
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
				return fmt.Errorf("error walking helm chart directory: %w", err)
			}

			relPath, err := filepath.Rel(c.path, path)
			if err != nil {
				return fmt.Errorf("error getting relative path: %w", err)
			}
			// Use the relative path to match against the provided filter.
			if match(relPath) {
				// Append the unmodified/absolute path to the matched files.
				matchedFiles = append(matchedFiles, path)
			} else {
				slog.Debug("skipping file", slog.String("file", relPath))
			}

			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("error reading helm chart directory: %w", err)
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
		return nil, fmt.Errorf("error converting values schema to JSON Schema: %w", err)
	}

	return jsonSchema, nil
}

func (c *ChartFiles) GetCRDs(match func(string) bool) ([][]byte, error) {
	if match == nil {
		return nil, ErrNoMatcher
	}

	matchedFiles := []string{}

	err := filepath.Walk(c.path,
		func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("error walking helm chart directory: %w", err)
			}

			relPath, err := filepath.Rel(c.path, path)
			if err != nil {
				return fmt.Errorf("error getting relative path: %w", err)
			}
			// Use the relative path to match against the provided filter.
			if match(relPath) {
				// Append the unmodified/absolute path to the matched files.
				matchedFiles = append(matchedFiles, path)
			} else {
				slog.Debug("skipping file", slog.String("file", relPath))
			}

			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("error reading helm chart directory: %w", err)
	}

	crdBytes := [][]byte{}

	if len(matchedFiles) == 0 {
		slog.Warn("no input files found for the CRD schema generator",
			slog.String("chart", c.TemplateOpts.ChartName),
			slog.String("path", c.path),
		)

		return crdBytes, nil
	}

	for _, f := range matchedFiles {
		//nolint:gosec // G304 not relevant for client-side generation.
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("error reading CRD file: %w", err)
		}

		crdBytes = append(crdBytes, b)
	}

	return crdBytes, nil
}

func (c *ChartFiles) Dispose() {
	if c.closer != nil {
		tryClose(c.closer)
	}
}
