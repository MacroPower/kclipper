package jsonschema

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	helmschema "github.com/dadav/helm-schema/pkg/schema"
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

// unmarshalHelmSchema unmarshals data (JSON or YAML) into a [helmschema.Schema].
// YAML is a superset of JSON, so this works for both formats.
func unmarshalHelmSchema(data []byte) (*helmschema.Schema, error) {
	var node yaml.Node

	err := yaml.Unmarshal(data, &node)
	if err != nil {
		return nil, fmt.Errorf("unmarshal YAML: %w", err)
	}

	hs := &helmschema.Schema{}

	err = hs.UnmarshalYAML(&node)
	if err != nil {
		return nil, fmt.Errorf("unmarshal helm schema: %w", err)
	}

	return hs, nil
}
