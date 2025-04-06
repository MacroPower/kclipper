package crd

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/MacroPower/kclipper/pkg/kube"
)

// ReaderGenerator reads CRDs from a given location and returns
// corresponding []*unstructured.Unstructured representations.
type ReaderGenerator struct{}

// NewReaderGenerator creates a new [ReaderGenerator].
func NewReaderGenerator() *ReaderGenerator {
	return &ReaderGenerator{}
}

func (g *ReaderGenerator) FromReader(r io.Reader) ([]*unstructured.Unstructured, error) {
	jsBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read: %w", err)
	}

	return g.FromData(jsBytes)
}

func (g *ReaderGenerator) FromData(data []byte) ([]*unstructured.Unstructured, error) {
	resources, err := kube.SplitYAML(data)
	if err != nil {
		return nil, fmt.Errorf("split yaml: %w", err)
	}

	crds := []*unstructured.Unstructured{}
	for _, r := range resources {
		if resourceIsCRD(r) {
			crds = append(crds, r)
		}
	}

	return crds, nil
}

func resourceIsCRD(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return false
	}

	if obj.GetAPIVersion() != CRDAPIVersion {
		return false
	}

	if obj.GetKind() != CRDKind {
		return false
	}

	return true
}
