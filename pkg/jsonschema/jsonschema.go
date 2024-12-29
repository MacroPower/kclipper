package jsonschema

import "strings"

type Generator interface {
	FromData(data []byte) ([]byte, error)
	FromPaths(paths ...string) ([]byte, error)
}

type GeneratorType string

const (
	AutoGeneratorType           GeneratorType = "AUTO"
	ValueInferenceGeneratorType GeneratorType = "VALUE-INFERENCE"
	URLGeneratorType            GeneratorType = "URL"
	PathGeneratorType           GeneratorType = "PATH"
	LocalPathGeneratorType      GeneratorType = "LOCAL-PATH"
	NoGeneratorType             GeneratorType = "NONE"
)

var GeneratorTypeEnum = []interface{}{
	AutoGeneratorType,
	ValueInferenceGeneratorType,
	URLGeneratorType,
	PathGeneratorType,
	LocalPathGeneratorType,
	NoGeneratorType,
}

// GetGenerator returns a [Generator] for the given [GeneratorType].
//
//nolint:ireturn
func GetGenerator(t GeneratorType) Generator {
	switch t {
	case AutoGeneratorType:
		return DefaultAutoGenerator
	case ValueInferenceGeneratorType:
		return DefaultValueInferenceGenerator
	case URLGeneratorType, PathGeneratorType, LocalPathGeneratorType:
		return DefaultReaderGenerator
	case NoGeneratorType:
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
	case string(PathGeneratorType):
		return PathGeneratorType
	case string(LocalPathGeneratorType):
		return LocalPathGeneratorType
	case string(NoGeneratorType):
		return NoGeneratorType
	default:
		return NoGeneratorType
	}
}
