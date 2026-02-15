package crd

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/macropower/kclipper/pkg/kube"
)

// FromPaths reads CRDs from the given file paths and returns the corresponding
// []kube.Object representation.
func FromPaths(paths ...string) ([]kube.Object, error) {
	if len(paths) == 0 {
		return nil, errors.New("no paths provided")
	}

	crds := []kube.Object{}
	for _, path := range paths {
		c, err := FromPath(path)
		if err != nil {
			return nil, fmt.Errorf("read CRDs from %s: %w", path, err)
		}

		crds = append(crds, c...)
	}

	return crds, nil
}

// FromPath reads CRDs from the given file path and returns the corresponding
// []kube.Object representation.
func FromPath(path string) ([]kube.Object, error) {
	//nolint:gosec // G304 not relevant for client-side generation.
	jsBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return FromReader(bytes.NewReader(jsBytes))
}
