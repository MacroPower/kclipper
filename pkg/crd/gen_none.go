package crd

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

var DefaultNoGenerator = NewNoGenerator()

// NoGenerator always returns empty lists.
type NoGenerator struct{}

// NewNoGenerator creates a new [NoGenerator].
func NewNoGenerator() *NoGenerator {
	return &NoGenerator{}
}

// FromData returns an empty list, regardless of the input data.
func (g *NoGenerator) FromData(_ []byte) ([]*unstructured.Unstructured, error) {
	return []*unstructured.Unstructured{}, nil
}

// FromPaths returns an empty list, regardless of the input paths.
func (g *NoGenerator) FromPaths(_ ...string) ([]*unstructured.Unstructured, error) {
	return []*unstructured.Unstructured{}, nil
}
