// Copyright (c) 2023 dadav, Licensed under the MIT License.
// Modifications Copyright (c) 2024-2025 Jacob Colvin
// Licensed under the Apache License, Version 2.0

package jsonschema

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	helmschema "github.com/dadav/helm-schema/pkg/schema"
	"github.com/dadav/helm-schema/pkg/util"
	"gopkg.in/yaml.v3"
)

var defaultValuesFileRegex = regexp.MustCompile(`^(.*/)?values\.ya?ml$`)

// DefaultValueInferenceGenerator is an opinionated [ValueInferenceGenerator].
var DefaultValueInferenceGenerator = NewValueInferenceGenerator(ValueInferenceConfig{
	SkipRequired:             true,
	SkipAdditionalProperties: true,
})

var _ FileGenerator = DefaultValueInferenceGenerator

type ValueInferenceConfig struct {
	// DefaultFileRegex matches files that set the `default` attribute in the JSON Schema.
	DefaultFileRegex          *regexp.Regexp
	UncommentYAMLBlocks       bool
	HelmDocsCompatibilityMode bool
	SkipTitle                 bool
	SkipDescription           bool
	SkipRequired              bool
	SkipDefault               bool
	SkipAdditionalProperties  bool
}

// ValueInferenceGenerator is a generator that infers a JSON Schema from one or
// more Helm values files.
type ValueInferenceGenerator struct {
	skipAutoGenerationConfig  *helmschema.SkipAutoGenerationConfig
	defaultFileRegex          *regexp.Regexp
	uncommentYAMLBlocks       bool
	helmDocsCompatibilityMode bool
}

// NewValueInferenceGenerator creates a new [ValueInferenceGenerator] using the
// given [ValueInferenceConfig].
func NewValueInferenceGenerator(c ValueInferenceConfig) *ValueInferenceGenerator {
	if c.DefaultFileRegex == nil {
		c.DefaultFileRegex = defaultValuesFileRegex
	}
	helmSkipAutoGenerationConfig := &helmschema.SkipAutoGenerationConfig{
		Title:                c.SkipTitle,
		Description:          c.SkipDescription,
		Required:             c.SkipRequired,
		Default:              c.SkipDefault,
		AdditionalProperties: c.SkipAdditionalProperties,
	}
	return &ValueInferenceGenerator{
		uncommentYAMLBlocks:       c.UncommentYAMLBlocks,
		helmDocsCompatibilityMode: c.HelmDocsCompatibilityMode,
		defaultFileRegex:          c.DefaultFileRegex,
		skipAutoGenerationConfig:  helmSkipAutoGenerationConfig,
	}
}

// FromPaths generates a JSON Schema from one or more file paths pointing to
// Helm values files. If multiple file paths are provided, the schemas are
// merged into a single schema, using defaults from the file matching
// DefaultFileRegex (usually `values.yaml`).
func (g *ValueInferenceGenerator) FromPaths(paths ...string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, errors.New("no file paths provided")
	}

	schemas := map[string]*helmschema.Schema{}
	for _, path := range paths {
		schema, err := g.schemaFromFilePath(path)
		if err != nil {
			return nil, fmt.Errorf("error creating schema from file path: %w", err)
		}
		schemas[path] = schema
	}

	mergedSchema := &helmschema.Schema{}
	for k, vs := range schemas {
		mergedSchema = mergeHelmSchemas(mergedSchema, vs, g.defaultFileRegex.MatchString(k))
	}

	if err := mergedSchema.Validate(); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	return marshalHelmSchema(mergedSchema)
}

func (g *ValueInferenceGenerator) FromData(data []byte) ([]byte, error) {
	schema, err := g.schemaFromData(data)
	if err != nil {
		return nil, fmt.Errorf("error creating schema from datum: %w", err)
	}

	mergedSchema := &helmschema.Schema{}
	mergedSchema = mergeHelmSchemas(mergedSchema, schema, true)

	return marshalHelmSchema(mergedSchema)
}

func (g *ValueInferenceGenerator) schemaFromFilePath(path string) (*helmschema.Schema, error) {
	valuesFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening values file: %w", err)
	}
	content, err := util.ReadFileAndFixNewline(valuesFile)
	if err != nil {
		return nil, fmt.Errorf("error reading values file: %w", err)
	}

	return g.schemaFromData(content)
}

func (g *ValueInferenceGenerator) schemaFromData(data []byte) (*helmschema.Schema, error) {
	// Check if a schema reference exists in the yaml file
	schemaRef := `# yaml-language-server: $schema=`
	if strings.Contains(string(data), schemaRef) {
		return nil, errors.New("schema reference already exists in values file")
	}

	var err error
	// Optional preprocessing
	if g.uncommentYAMLBlocks {
		// Remove comments from valid yaml
		data, err = util.RemoveCommentsFromYaml(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed uncommenting yaml: %w", err)
		}
	}

	var values yaml.Node
	err = yaml.Unmarshal(data, &values)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal values yaml: %w", err)
	}

	keepFullComment := false
	keepHelmDocsPrefix := false
	removeGlobal := false
	valuesSchema := helmschema.YamlToSchema("", &values, keepFullComment, g.helmDocsCompatibilityMode,
		keepHelmDocsPrefix, removeGlobal, g.skipAutoGenerationConfig, nil)

	if err := updateHelmSchema(valuesSchema, allowAdditionalProperties); err != nil {
		return nil, fmt.Errorf("failed setting allowAdditionalProperties on schema: %w", err)
	}

	return valuesSchema, nil
}

func allowAdditionalProperties(s *helmschema.Schema) error {
	if s.Type.Matches("object") {
		s.AdditionalProperties = true
	}

	return nil
}

func marshalHelmSchema(s *helmschema.Schema) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("error validating schema: %w", err)
	}

	jsonSchema, err := s.ToJson()
	if err != nil {
		return nil, fmt.Errorf("error converting schema to JSON Schema: %w", err)
	}

	return jsonSchema, nil
}

func updateHelmSchema(s *helmschema.Schema, fn func(s *helmschema.Schema) error) error {
	if err := fn(s); err != nil {
		return err
	}
	for _, v := range s.Properties {
		if err := fn(v); err != nil {
			return err
		}
	}
	if s.Items != nil {
		if err := fn(s.Items); err != nil {
			return err
		}
	}

	if s.AnyOf != nil {
		for _, v := range s.AnyOf {
			if err := fn(v); err != nil {
				return err
			}
		}
	}
	if s.OneOf != nil {
		for _, v := range s.OneOf {
			if err := fn(v); err != nil {
				return err
			}
		}
	}
	if s.AllOf != nil {
		for _, v := range s.AllOf {
			if err := fn(v); err != nil {
				return err
			}
		}
	}
	if s.If != nil {
		if err := fn(s.If); err != nil {
			return err
		}
	}
	if s.Else != nil {
		if err := fn(s.Else); err != nil {
			return err
		}
	}
	if s.Then != nil {
		if err := fn(s.Then); err != nil {
			return err
		}
	}
	if s.Not != nil {
		if err := fn(s.Not); err != nil {
			return err
		}
	}
	return nil
}

func mergeHelmSchemas(dest, src *helmschema.Schema, setDefaults bool) *helmschema.Schema {
	if dest == nil {
		return mergeHelmSchemas(&helmschema.Schema{}, src, setDefaults)
	}
	if src == nil {
		return mergeHelmSchemas(&helmschema.Schema{}, dest, setDefaults)
	}

	if setDefaults {
		dest.Default = src.Default
	}

	// Resolve simple fields by favoring the fields from 'src' if they're provided
	if !src.Type.IsEmpty() {
		dest.Type = src.Type
	}
	if src.Schema != "" {
		dest.Schema = src.Schema
	}
	if src.MultipleOf != nil {
		dest.MultipleOf = src.MultipleOf
	}
	if src.Maximum != nil {
		dest.Maximum = src.Maximum
	}
	if src.Minimum != nil {
		dest.Minimum = src.Minimum
	}
	if src.MaxLength != nil {
		dest.MaxLength = src.MaxLength
	}
	if src.MinLength != nil {
		dest.MinLength = src.MinLength
	}
	if src.Pattern != "" {
		dest.Pattern = src.Pattern
	}
	if src.MaxItems != nil {
		dest.MaxItems = src.MaxItems
	}
	if src.MinItems != nil {
		dest.MinItems = src.MinItems
	}
	if src.ExclusiveMaximum != nil {
		dest.ExclusiveMaximum = src.ExclusiveMaximum
	}
	if src.ExclusiveMinimum != nil {
		dest.ExclusiveMinimum = src.ExclusiveMinimum
	}
	if src.PatternProperties != nil {
		dest.PatternProperties = src.PatternProperties
	}
	if src.Title != "" {
		dest.Title = src.Title
	}
	if src.Description != "" {
		dest.Description = src.Description
	}
	if src.ReadOnly {
		dest.ReadOnly = src.ReadOnly
	}
	if src.AdditionalProperties != nil {
		dest.AdditionalProperties = src.AdditionalProperties
	}
	if src.Id != "" {
		dest.Id = src.Id
	}

	// Merge 'enum' field (assuming that maintaining order doesn't matter)
	dest.Enum = append(dest.Enum, src.Enum...)

	// Recursive calls for nested structures
	if src.Properties != nil {
		if dest.Properties == nil {
			dest.Properties = make(map[string]*helmschema.Schema)
		}
		for propName, srcPropSchema := range src.Properties {
			if destPropSchema, exists := dest.Properties[propName]; exists {
				dest.Properties[propName] = mergeHelmSchemas(destPropSchema, srcPropSchema, setDefaults)
			} else {
				dest.Properties[propName] = mergeHelmSchemas(&helmschema.Schema{}, srcPropSchema, setDefaults)
			}
		}
	}

	// Merge 'items' if they exist (assuming they're not arrays)
	if src.Items != nil {
		dest.Items = mergeHelmSchemas(dest.Items, src.Items, setDefaults)
	}

	var items *helmschema.Schema
	for _, s := range append(dest.AllOf, src.AllOf...) {
		items = mergeHelmSchemas(items, s, setDefaults)
		dest.AllOf = nil
	}
	for _, s := range append(dest.AnyOf, src.AnyOf...) {
		items = mergeHelmSchemas(items, s, setDefaults)
		dest.AnyOf = nil
	}
	for _, s := range append(dest.OneOf, src.OneOf...) {
		items = mergeHelmSchemas(items, s, setDefaults)
		dest.OneOf = nil
	}
	if items != nil {
		dest = mergeHelmSchemas(dest, items, setDefaults)
	}

	if src.If != nil {
		dest.If = mergeHelmSchemas(dest.If, src.If, setDefaults)
	}
	if src.Else != nil {
		dest.Else = mergeHelmSchemas(dest.Else, src.Else, setDefaults)
	}
	if src.Then != nil {
		dest.Then = mergeHelmSchemas(dest.Then, src.Then, setDefaults)
	}
	if src.Not != nil {
		dest.Not = mergeHelmSchemas(dest.Not, src.Not, setDefaults)
	}

	return dest
}
