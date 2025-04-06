package crd

import (
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GeneratorType defines the type of CRD generator to use.
type GeneratorType string

const (
	// GeneratorTypeDefault is the default generator type.
	GeneratorTypeDefault GeneratorType = ""
	// GeneratorTypeAuto tries to automatically detect the best generator.
	GeneratorTypeAuto GeneratorType = "AUTO"
	// GeneratorTypeTemplate uses a template-based generator.
	GeneratorTypeTemplate GeneratorType = "TEMPLATE"
	// GeneratorTypeChartPath uses a chart path-based generator.
	GeneratorTypeChartPath GeneratorType = "CHART-PATH"
	// GeneratorTypePath uses either a local path or a URL-based generator.
	GeneratorTypePath GeneratorType = "PATH"
	// GeneratorTypeNone skips CRD generation.
	GeneratorTypeNone GeneratorType = "NONE"

	CRDAPIVersion string = "apiextensions.k8s.io/v1"
	CRDKind       string = "CustomResourceDefinition"
)

var (
	// GeneratorTypeEnum lists all valid CRD generator types.
	GeneratorTypeEnum = []any{
		GeneratorTypeAuto,
		GeneratorTypeTemplate,
		GeneratorTypeChartPath,
		GeneratorTypePath,
		GeneratorTypeNone,
	}

	// ErrInvalidFormat indicates an unexpected or invalid format was encountered.
	ErrInvalidFormat = errors.New("invalid format")

	// ErrGenerateKCL indicates an error occurred during KCL generation.
	ErrGenerateKCL = errors.New("failed to generate KCL")
)

func GetGeneratorType(t string) GeneratorType {
	switch strings.TrimSpace(strings.ToUpper(t)) {
	case string(GeneratorTypeAuto):
		return GeneratorTypeAuto
	case string(GeneratorTypeTemplate):
		return GeneratorTypeTemplate
	case string(GeneratorTypeChartPath):
		return GeneratorTypeChartPath
	case string(GeneratorTypePath):
		return GeneratorTypePath
	case string(GeneratorTypeNone):
		return GeneratorTypeNone
	default:
		return GeneratorTypeDefault
	}
}

// SplitCRDVersions separates a CRD with multiple versions into individual CRDs.
// Returns a map where the key is the version name and the value is the CRD object.
func SplitCRDVersions(crd *unstructured.Unstructured) (map[string]unstructured.Unstructured, error) {
	spec, ok := crd.Object["spec"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: invalid spec field", ErrInvalidFormat)
	}

	versions, ok := spec["versions"].([]any)
	if !ok {
		return nil, fmt.Errorf("%w: invalid spec.versions field", ErrInvalidFormat)
	}

	crdVersions := make(map[string]unstructured.Unstructured, len(versions))
	for _, version := range versions {
		version, ok := version.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: invalid spec.versions[] field", ErrInvalidFormat)
		}

		versionName, ok := version["name"].(string)
		if !ok {
			return nil, fmt.Errorf("%w: invalid spec.versions[].name field", ErrInvalidFormat)
		}

		crdVersion := crd.DeepCopy()
		versionSpec, ok := crdVersion.Object["spec"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: invalid spec field after deep copy", ErrInvalidFormat)
		}

		versionSpec["versions"] = []any{version}
		crdVersions[versionName] = *crdVersion
	}

	return crdVersions, nil
}
