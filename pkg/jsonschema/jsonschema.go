package jsonschema

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"go.jacobcolvin.com/x/jsonschema"
	"sigs.k8s.io/yaml"
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

// unmarshalSchema unmarshals data (JSON or YAML) into a [jsonschema.Schema].
// YAML is a superset of JSON, so this works for both formats.
func unmarshalSchema(data []byte) (*jsonschema.Schema, error) {
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("convert YAML to JSON: %w", err)
	}

	s := &jsonschema.Schema{}

	err = json.Unmarshal(jsonData, s)
	if err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}

	return s, nil
}

// marshalSchema marshals a [jsonschema.Schema] to indented JSON.
func marshalSchema(s *jsonschema.Schema) ([]byte, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	return data, nil
}

// stripUnknownMembers removes non-keyword members captured during
// unmarshaling from s and all nested sub-schemas. Schemas read from external
// sources can carry arbitrary unknown members (e.g. wrapper keys in malformed
// vendored schemas); they validate nothing under draft-07 and mislead anyone
// reading the schema as if they were constraints.
func stripUnknownMembers(s *jsonschema.Schema) {
	//nolint:errcheck // The visitor never returns an error.
	_ = jsonschema.Walk(s, func(_ jsonschema.Location, s *jsonschema.Schema) error {
		s.Extra = nil

		return nil
	})
}
