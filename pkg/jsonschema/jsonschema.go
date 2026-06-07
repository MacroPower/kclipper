package jsonschema

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"

	gojsonschema "github.com/google/jsonschema-go/jsonschema"
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

	// Type names permitted by JSON Schema.
	validTypeNames = map[string]bool{
		"array":   true,
		"boolean": true,
		"integer": true,
		"null":    true,
		"number":  true,
		"object":  true,
		"string":  true,
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

// validateSchema checks that s is structurally usable as a JSON Schema,
// validating type names on s and all nested sub-schemas.
//
// It deliberately validates only type names, tolerating schema authoring
// opinions (e.g. format on non-string types) that are violated by many
// real-world schemas, like those derived from the Kubernetes API. Schemas
// that kclipper reads and converts must be tolerated, even if stricter
// generators would not produce them.
func validateSchema(s *gojsonschema.Schema) error {
	err := walkSchema(s, func(s *gojsonschema.Schema) error {
		if s.Type != "" && !validTypeNames[s.Type] {
			return fmt.Errorf("invalid type: %q", s.Type)
		}

		for _, t := range s.Types {
			if !validTypeNames[t] {
				return fmt.Errorf("invalid type: %q", t)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	return nil
}

// unmarshalSchema unmarshals data (JSON or YAML) into a [gojsonschema.Schema].
// YAML is a superset of JSON, so this works for both formats.
func unmarshalSchema(data []byte) (*gojsonschema.Schema, error) {
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("convert YAML to JSON: %w", err)
	}

	s := &gojsonschema.Schema{}

	err = json.Unmarshal(jsonData, s)
	if err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}

	return s, nil
}

// marshalSchema marshals a [gojsonschema.Schema] to indented JSON.
func marshalSchema(s *gojsonschema.Schema) ([]byte, error) {
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
func stripUnknownMembers(s *gojsonschema.Schema) {
	//nolint:errcheck // The visitor never returns an error.
	_ = walkSchema(s, func(s *gojsonschema.Schema) error {
		s.Extra = nil

		return nil
	})
}

// walkSchema recursively applies fn to s and all nested sub-schemas.
func walkSchema(s *gojsonschema.Schema, fn func(s *gojsonschema.Schema) error) error {
	if s == nil {
		return nil
	}

	err := fn(s)
	if err != nil {
		return err
	}

	for _, sub := range [...]*gojsonschema.Schema{
		s.Items, s.AdditionalItems, s.Contains, s.UnevaluatedItems,
		s.AdditionalProperties, s.PropertyNames, s.UnevaluatedProperties,
		s.Not, s.If, s.Then, s.Else, s.ContentSchema,
	} {
		err = walkSchema(sub, fn)
		if err != nil {
			return err
		}
	}

	for _, subs := range [...][]*gojsonschema.Schema{
		s.PrefixItems, s.ItemsArray, s.AllOf, s.AnyOf, s.OneOf,
	} {
		for _, sub := range subs {
			err = walkSchema(sub, fn)
			if err != nil {
				return err
			}
		}
	}

	for _, subs := range [...]map[string]*gojsonschema.Schema{
		s.Defs, s.Definitions, s.DependencySchemas, s.Properties,
		s.PatternProperties, s.DependentSchemas,
	} {
		for _, sub := range subs {
			err = walkSchema(sub, fn)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
