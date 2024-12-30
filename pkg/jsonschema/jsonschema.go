package jsonschema

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	jsv6 "github.com/santhosh-tekuri/jsonschema/v6"
)

type FileGenerator interface {
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

// GetGenerator returns a [FileGenerator] for the given [GeneratorType].
//
//nolint:ireturn
func GetGenerator(t GeneratorType) FileGenerator {
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

var (
	jsonOrYAMLValuesRegex = regexp.MustCompile(`(\.json|values.*\.ya?ml)$`)
	yamlValuesRegex       = regexp.MustCompile(`values.*\.ya?ml$`)
)

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
		//nolint:gocritic
		return func(s string) bool {
			return isJSONFile(s)
		}
	}
}

func isYAMLFile(f string) bool {
	return filepath.Ext(f) == ".yaml" || filepath.Ext(f) == ".yml"
}

func isJSONFile(f string) bool {
	return filepath.Ext(f) == ".json"
}

// Validate ensures that the given JSON data is a valid JSON Schema. It returns
// true if the JSON data is a valid JSON Schema, otherwise it returns false
// as well as an error describing the validation failure.
func Validate(jsonData []byte) (bool, error) {
	schema, err := jsv6.UnmarshalJSON(bytes.NewReader(jsonData))
	if err != nil {
		return false, fmt.Errorf("failed unmarshaling JSON Schema: %w", err)
	}

	compiler := jsv6.NewCompiler()
	if err := compiler.AddResource("schema.json", schema); err != nil {
		return false, fmt.Errorf("failed to add JSON Schema to validator: %w", err)
	}
	cSchema, err := compiler.Compile("schema.json")
	if err != nil {
		return false, fmt.Errorf("failed to validate JSON Schema: %w", err)
	}
	if len(cSchema.Properties) == 0 {
		return false, fmt.Errorf("no properties found on JSON Schema: %#v", schema)
	}

	return true, nil
}
