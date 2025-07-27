// Copyright (c) 2023 dadav, Licensed under the MIT License.
// Modifications Copyright (c) 2024-2025 Jacob Colvin
// Licensed under the Apache License, Version 2.0.

package jsonschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/dadav/helm-schema/pkg/util"
	"gopkg.in/yaml.v3"

	helmschema "github.com/dadav/helm-schema/pkg/schema"
)

var (
	// DefaultValueInferenceGenerator is an opinionated [ValueInferenceGenerator].
	DefaultValueInferenceGenerator = NewValueInferenceGenerator(&ValueInferenceConfig{
		SkipRequired: true,
	})

	// DefaultFileRegex matches files that set the `default` attribute in the JSON Schema.
	defaultValuesFileRegex = regexp.MustCompile(`^(.*/)?values\.ya?ml$`)

	_ FileGenerator = DefaultValueInferenceGenerator
)

type ValueInferenceConfig struct {
	// Consider yaml which is commented out.
	UncommentYAMLBlocks bool `json:"uncommentYAMLBlocks,omitempty"`
	// Parse and use helm-docs comments.
	HelmDocsCompatibilityMode bool `json:"helmDocsCompatibilityMode,omitempty"`
	// Keep the helm-docs prefix (--) in the schema.
	KeepHelmDocsPrefix bool `json:"keepHelmDocsPrefix,omitempty"`
	// Keep the whole leading comment (default: cut at empty line).
	KeepFullComment bool `json:"keepFullComment,omitempty"`
	// Remove the global key from the schema.
	RemoveGlobal bool `json:"removeGlobal,omitempty"`
	// Skip auto-generation of Title.
	SkipTitle bool `json:"skipTitle,omitempty"`
	// Skip auto-generation of Description.
	SkipDescription bool `json:"skipDescription,omitempty"`
	// Skip auto-generation of Required.
	SkipRequired bool `json:"skipRequired,omitempty"`
	// Skip auto-generation of Default.
	SkipDefault bool `json:"skipDefault,omitempty"`
	// Skip auto-generation of AdditionalProperties.
	SkipAdditionalProperties bool `json:"skipAdditionalProperties,omitempty"`
}

// ValueInferenceGenerator is a generator that infers a JSON Schema from one or
// more Helm values files.
type ValueInferenceGenerator struct {
	skipAutoGenerationConfig *helmschema.SkipAutoGenerationConfig
	defaultFileRegex         *regexp.Regexp
	config                   *ValueInferenceConfig
}

// NewValueInferenceGenerator creates a new [ValueInferenceGenerator] using the
// given [ValueInferenceConfig].
func NewValueInferenceGenerator(c *ValueInferenceConfig) *ValueInferenceGenerator {
	helmSkipAutoGenerationConfig := &helmschema.SkipAutoGenerationConfig{
		Title:                c.SkipTitle,
		Description:          c.SkipDescription,
		Required:             c.SkipRequired,
		Default:              c.SkipDefault,
		AdditionalProperties: c.SkipAdditionalProperties,
	}

	return &ValueInferenceGenerator{
		defaultFileRegex:         defaultValuesFileRegex,
		skipAutoGenerationConfig: helmSkipAutoGenerationConfig,
		config:                   c,
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

	slices.Sort(paths)

	schemas := map[string]*helmschema.Schema{}

	for _, path := range paths {
		schema, err := g.schemaFromFilePath(path)
		if err != nil {
			return nil, fmt.Errorf("error creating schema from file path: %w", err)
		}

		schemas[path] = schema
	}

	mergedSchema := &helmschema.Schema{}
	for _, k := range paths {
		mergedSchema = mergeHelmSchemas(mergedSchema, schemas[k], g.defaultFileRegex.MatchString(k))
	}

	err := mergedSchema.Validate()
	if err != nil {
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
	//nolint:gosec // G304 not relevant for client-side generation.
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
	// Check if a schema reference exists in the yaml file.
	schemaRef := `# yaml-language-server: $schema=`
	if strings.Contains(string(data), schemaRef) {
		return nil, errors.New("schema reference already exists in values file")
	}

	var err error
	// Optional preprocessing.
	if g.config.UncommentYAMLBlocks {
		// Remove comments from valid yaml.
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

	valuesSchema := helmschema.YamlToSchema("", &values, g.config.KeepFullComment, g.config.HelmDocsCompatibilityMode,
		g.config.KeepHelmDocsPrefix, g.config.RemoveGlobal, g.skipAutoGenerationConfig, nil)

	err = updateHelmSchema(valuesSchema, allowAdditionalProperties)
	if err != nil {
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
	err := s.Validate()
	if err != nil {
		return nil, fmt.Errorf("error validating schema: %w", err)
	}

	jsonSchema, err := s.ToJson()
	if err != nil {
		return nil, fmt.Errorf("error converting schema to JSON Schema: %w", err)
	}

	return jsonSchema, nil
}

func updateHelmSchema(s *helmschema.Schema, fn func(s *helmschema.Schema) error) error {
	err := fn(s)
	if err != nil {
		return err
	}

	for _, v := range s.Properties {
		err := updateHelmSchema(v, fn)
		if err != nil {
			return err
		}
	}

	if s.Items != nil {
		err := fn(s.Items)
		if err != nil {
			return err
		}
	}

	if s.AnyOf != nil {
		for _, v := range s.AnyOf {
			err := updateHelmSchema(v, fn)
			if err != nil {
				return err
			}
		}
	}

	if s.OneOf != nil {
		for _, v := range s.OneOf {
			err := updateHelmSchema(v, fn)
			if err != nil {
				return err
			}
		}
	}

	if s.AllOf != nil {
		for _, v := range s.AllOf {
			err := updateHelmSchema(v, fn)
			if err != nil {
				return err
			}
		}
	}

	if s.If != nil {
		err := fn(s.If)
		if err != nil {
			return err
		}
	}

	if s.Else != nil {
		err := fn(s.Else)
		if err != nil {
			return err
		}
	}

	if s.Then != nil {
		err := fn(s.Then)
		if err != nil {
			return err
		}
	}

	if s.Not != nil {
		err := fn(s.Not)
		if err != nil {
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

	// Resolve simple fields by favoring the fields from 'src' if they're provided.
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

	if src.Id != "" {
		dest.Id = src.Id
	}

	// Merge 'enum' field (assuming that maintaining order doesn't matter).
	dest.Enum = slices.Compact(append(dest.Enum, src.Enum...))

	dest.Required = helmschema.BoolOrArrayOfString{
		Bool:    dest.Required.Bool || src.Required.Bool,
		Strings: intersectStringSlices(dest.Required.Strings, src.Required.Strings),
	}

	// Recursive calls for nested structures.
	if src.Properties != nil {
		if dest.Properties == nil {
			dest.Properties = make(map[string]*helmschema.Schema)
		}

		propKeys := []string{}
		for k := range src.Properties {
			propKeys = append(propKeys, k)
		}

		slices.Sort(propKeys)

		for _, k := range propKeys {
			if destPropSchema, exists := dest.Properties[k]; exists {
				dest.Properties[k] = mergeHelmSchemas(destPropSchema, src.Properties[k], setDefaults)
			} else {
				dest.Properties[k] = mergeHelmSchemas(&helmschema.Schema{}, src.Properties[k], setDefaults)
			}
		}
	}

	if src.AdditionalProperties != nil {
		err := mergeSchemaAdditionalProperties(dest, src, setDefaults)
		if err != nil {
			dest.AdditionalProperties = true
		}
	}

	// Merge 'items' if they exist (assuming they're not arrays).
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
		dest = mergeHelmSchemas(dest, src.If, setDefaults)
	}

	if src.Else != nil {
		dest = mergeHelmSchemas(dest, src.Else, setDefaults)
	}

	if src.Then != nil {
		dest = mergeHelmSchemas(dest, src.Then, setDefaults)
	}

	if src.Not != nil {
		dest = mergeHelmSchemas(dest, src.Not, setDefaults)
	}

	return dest
}

func intersectStringSlices(a, b []string) []string {
	intersection := []string{}

	for _, x := range a {
		if slices.Contains(b, x) {
			intersection = append(intersection, x)
		}
	}

	return intersection
}

func mergeSchemaAdditionalProperties(dest, src *helmschema.Schema, setDefaults bool) error {
	//nolint:revive // Boolean literal used due to SchemaOrBool type.
	if src.AdditionalProperties == true || src.AdditionalProperties == false {
		dest.AdditionalProperties = src.AdditionalProperties

		return nil
	}

	srcData, err := json.Marshal(src.AdditionalProperties)
	if err != nil {
		return fmt.Errorf("failed to marshal src additional properties: %w", err)
	}

	destData, err := json.Marshal(dest.AdditionalProperties)
	if err != nil {
		return fmt.Errorf("failed to marshal dest additional properties: %w", err)
	}

	srcSubSchema := &helmschema.Schema{}

	var (
		jsonSrcNode  yaml.Node
		jsonDestNode yaml.Node
	)

	err = yaml.Unmarshal(srcData, &jsonSrcNode)
	if err != nil {
		return fmt.Errorf("failed to unmarshal src additional properties: %w", err)
	}

	err = srcSubSchema.UnmarshalYAML(&jsonSrcNode)
	if err != nil {
		return fmt.Errorf("failed to unmarshal src additional properties: %w", err)
	}

	destSubSchema := &helmschema.Schema{}

	err = yaml.Unmarshal(destData, &jsonDestNode)
	if err != nil {
		return fmt.Errorf("failed to unmarshal dest additional properties: %w", err)
	}

	err = destSubSchema.UnmarshalYAML(&jsonDestNode)
	if err != nil {
		return fmt.Errorf("failed to unmarshal dest additional properties: %w", err)
	}

	subSchema := mergeHelmSchemas(destSubSchema, srcSubSchema, setDefaults)
	err = subSchema.Validate()
	if err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	dest.AdditionalProperties = subSchema

	return nil
}
