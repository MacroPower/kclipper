package jsonschema

import (
	"path/filepath"
	"regexp"
	"strings"
)

type (
	GeneratorType string
	ValidatorType string
)

const (
	DefaultGeneratorType        GeneratorType = ""
	AutoGeneratorType           GeneratorType = "AUTO"
	ValueInferenceGeneratorType GeneratorType = "VALUE-INFERENCE"
	URLGeneratorType            GeneratorType = "URL"
	ChartPathGeneratorType      GeneratorType = "CHART-PATH"
	LocalPathGeneratorType      GeneratorType = "LOCAL-PATH"
	NoGeneratorType             GeneratorType = "NONE"

	DefaultValidatorType ValidatorType = ""
	KCLValidatorType     ValidatorType = "KCL"
	HelmValidatorType    ValidatorType = "HELM"
)

var (
	GeneratorTypeEnum = []any{
		AutoGeneratorType,
		ValueInferenceGeneratorType,
		URLGeneratorType,
		ChartPathGeneratorType,
		LocalPathGeneratorType,
		NoGeneratorType,
	}

	ValidatorTypeEnum = []any{
		KCLValidatorType,
		HelmValidatorType,
	}

	jsonOrYAMLValuesRegex = regexp.MustCompile(`(\.json|values.*\.ya?ml)$`)
	yamlValuesRegex       = regexp.MustCompile(`values.*\.ya?ml$`)
)

type FileGenerator interface {
	FromPaths(paths ...string) ([]byte, error)
}

// GetGenerator returns a [FileGenerator] for the given [GeneratorType].
//
//nolint:ireturn // Multiple concrete types.
func GetGenerator(t GeneratorType) FileGenerator {
	switch t {
	case AutoGeneratorType:
		return DefaultAutoGenerator
	case ValueInferenceGeneratorType:
		return DefaultValueInferenceGenerator
	case URLGeneratorType, ChartPathGeneratorType, LocalPathGeneratorType:
		return DefaultReaderGenerator
	case DefaultGeneratorType, NoGeneratorType:
		return DefaultNoGenerator
	default:
		return DefaultNoGenerator
	}
}

func GetGeneratorType(t string) GeneratorType {
	switch strings.TrimSpace(strings.ToUpper(t)) {
	case string(AutoGeneratorType):
		return AutoGeneratorType
	case string(ValueInferenceGeneratorType):
		return ValueInferenceGeneratorType
	case string(URLGeneratorType):
		return URLGeneratorType
	case string(ChartPathGeneratorType):
		return ChartPathGeneratorType
	case string(LocalPathGeneratorType):
		return LocalPathGeneratorType
	case string(NoGeneratorType):
		return NoGeneratorType
	default:
		return DefaultGeneratorType
	}
}

func GetValidatorType(t string) ValidatorType {
	switch strings.TrimSpace(strings.ToUpper(t)) {
	case string(KCLValidatorType):
		return KCLValidatorType
	case string(HelmValidatorType):
		return HelmValidatorType
	default:
		return DefaultValidatorType
	}
}

func GetFileFilter(t GeneratorType) func(string) bool {
	switch t {
	case AutoGeneratorType:
		return func(s string) bool {
			return jsonOrYAMLValuesRegex.MatchString(s)
		}

	case ValueInferenceGeneratorType:
		return func(s string) bool {
			return yamlValuesRegex.MatchString(s)
		}

	default:
		return isJSONFile
	}
}

func isYAMLFile(f string) bool {
	return filepath.Ext(f) == ".yaml" || filepath.Ext(f) == ".yml"
}

func isJSONFile(f string) bool {
	return filepath.Ext(f) == ".json"
}
