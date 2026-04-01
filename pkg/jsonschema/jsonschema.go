package jsonschema

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"go.jacobcolvin.com/niceyaml"

	gjs "github.com/google/jsonschema-go/jsonschema"
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

	generatorTypes = map[string]GeneratorType{
		string(AutoGeneratorType):           AutoGeneratorType,
		string(ValueInferenceGeneratorType): ValueInferenceGeneratorType,
		string(URLGeneratorType):            URLGeneratorType,
		string(ChartPathGeneratorType):      ChartPathGeneratorType,
		string(LocalPathGeneratorType):      LocalPathGeneratorType,
		string(NoGeneratorType):             NoGeneratorType,
	}

	validatorTypes = map[string]ValidatorType{
		string(KCLValidatorType):  KCLValidatorType,
		string(HelmValidatorType): HelmValidatorType,
	}
)

// FileGenerator generates JSON Schema content from file paths.
// See [AutoGenerator] for an implementation.
type FileGenerator interface {
	FromPaths(paths ...string) ([]byte, error)
}

// GetGeneratorType returns the [GeneratorType] matching the given string,
// or [DefaultGeneratorType] if no match is found.
func GetGeneratorType(t string) GeneratorType {
	if gt, ok := generatorTypes[strings.TrimSpace(strings.ToUpper(t))]; ok {
		return gt
	}

	return DefaultGeneratorType
}

// GetValidatorType returns the [ValidatorType] matching the given string,
// or [DefaultValidatorType] if no match is found.
func GetValidatorType(t string) ValidatorType {
	if vt, ok := validatorTypes[strings.TrimSpace(strings.ToUpper(t))]; ok {
		return vt
	}

	return DefaultValidatorType
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

// unmarshalSchema unmarshals data (JSON or YAML) into a [gjs.Schema].
// YAML is a superset of JSON, so this works for both formats.
func unmarshalSchema(data []byte) (*gjs.Schema, error) {
	var v any

	src := niceyaml.NewSourceFromBytes(data)

	dec, err := src.Decoder()
	if err != nil {
		return nil, fmt.Errorf("unmarshal YAML: %w", err)
	}

	for _, doc := range dec.Documents() {
		err = doc.Decode(&v)
		if err != nil {
			return nil, fmt.Errorf("unmarshal YAML: %w", err)
		}

		break
	}

	jsonData, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal to JSON: %w", err)
	}

	s := &gjs.Schema{}

	err = json.Unmarshal(jsonData, s)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON Schema: %w", err)
	}

	return s, nil
}
