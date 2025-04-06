package crd

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var DefaultFileGenerator = NewFileGenerator()

// FileGenerator reads CRDs from file paths and returns corresponding
// []*unstructured.Unstructured representations.
type FileGenerator struct {
	*ReaderGenerator
}

func NewFileGenerator() *FileGenerator {
	return &FileGenerator{
		ReaderGenerator: NewReaderGenerator(),
	}
}

// FromPaths reads CRDs from the given file paths and returns the corresponding
// []*unstructured.Unstructured representation.
func (g *FileGenerator) FromPaths(paths ...string) ([]*unstructured.Unstructured, error) {
	if len(paths) == 0 {
		return nil, errors.New("no paths provided")
	}

	if len(paths) == 1 {
		return g.FromPath(paths[0])
	}

	crds := []*unstructured.Unstructured{}
	for _, path := range paths {
		c, err := g.FromPath(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read CRDs from %s: %w", path, err)
		}
		crds = append(crds, c...)
	}

	return crds, nil
}

// FromPath reads CRDs from the given file path and returns the corresponding
// []*unstructured.Unstructured representation.
func (g *FileGenerator) FromPath(path string) ([]*unstructured.Unstructured, error) {
	//nolint:gosec // G304 not relevant for client-side generation.
	jsBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return g.FromReader(bytes.NewReader(jsBytes))
}
