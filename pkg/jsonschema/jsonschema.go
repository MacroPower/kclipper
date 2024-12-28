package jsonschema

type Generator interface {
	FromData(data []byte) ([]byte, error)
	FromPath(paths string) ([]byte, error)
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

// GetGenerator returns a new instance of the generator type.
//
//nolint:ireturn // Maybe?
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
