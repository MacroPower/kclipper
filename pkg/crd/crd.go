package crd

import (
	"fmt"
	"strings"

	"github.com/macropower/kclipper/pkg/kclerrors"
	"github.com/macropower/kclipper/pkg/kube"
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

	generatorTypes = map[string]GeneratorType{
		string(GeneratorTypeAuto):      GeneratorTypeAuto,
		string(GeneratorTypeTemplate):  GeneratorTypeTemplate,
		string(GeneratorTypeChartPath): GeneratorTypeChartPath,
		string(GeneratorTypePath):      GeneratorTypePath,
		string(GeneratorTypeNone):      GeneratorTypeNone,
	}
)

// GetGeneratorType returns the [GeneratorType] matching the given string, or
// [GeneratorTypeDefault] if unrecognized.
func GetGeneratorType(t string) GeneratorType {
	if gt, ok := generatorTypes[strings.TrimSpace(strings.ToUpper(t))]; ok {
		return gt
	}

	return GeneratorTypeDefault
}

// SplitCRDVersions separates a CRD with multiple versions into individual CRDs.
// Returns a map where the key is the version name and the value is the CRD object.
func SplitCRDVersions(crd kube.Object) (map[string]kube.Object, error) {
	spec, ok := crd["spec"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: invalid spec field", kclerrors.ErrInvalidFormat)
	}

	versions, ok := spec["versions"].([]any)
	if !ok {
		return nil, fmt.Errorf("%w: invalid spec.versions field", kclerrors.ErrInvalidFormat)
	}

	crdVersions := make(map[string]kube.Object, len(versions))
	for _, version := range versions {
		version, ok := version.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: invalid spec.versions[] field", kclerrors.ErrInvalidFormat)
		}

		versionName, ok := version["name"].(string)
		if !ok {
			return nil, fmt.Errorf("%w: invalid spec.versions[].name field", kclerrors.ErrInvalidFormat)
		}

		crdVersion := crd.DeepCopy()
		versionSpec, ok := crdVersion["spec"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: invalid spec field after deep copy", kclerrors.ErrInvalidFormat)
		}

		versionSpec["versions"] = []any{version}
		crdVersions[versionName] = crdVersion
	}

	return crdVersions, nil
}
