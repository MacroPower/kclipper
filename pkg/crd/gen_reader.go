package crd

import (
	"fmt"
	"io"

	"github.com/macropower/kclipper/pkg/kube"
)

// FromReader reads CRDs from a reader and returns the corresponding
// []kube.Object representation.
func FromReader(r io.Reader) ([]kube.Object, error) {
	jsBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	return FromData(jsBytes)
}

// FromData reads CRDs from raw bytes and returns the corresponding
// []kube.Object representation.
func FromData(data []byte) ([]kube.Object, error) {
	resources, err := kube.SplitYAML(data)
	if err != nil {
		return nil, fmt.Errorf("split yaml: %w", err)
	}

	crds := []kube.Object{}
	for _, r := range resources {
		if r.IsCRD() {
			crds = append(crds, r)
		}
	}

	return crds, nil
}
