package jsonschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	gojsonschema "github.com/google/jsonschema-go/jsonschema"

	"github.com/macropower/kclipper/pkg/kclgen"
)

// ConvertToKCLSchema converts a JSON schema to a KCL schema.
func ConvertToKCLSchema(jsonSchemaData []byte, removeDefaults bool) ([]byte, error) {
	fixedJSONSchema, err := ConvertToKCLCompatibleJSONSchema(jsonSchemaData)
	if err != nil {
		return nil, fmt.Errorf("convert to KCL compatible JSON schema: %w", err)
	}

	kclSchema := &bytes.Buffer{}
	err = kclgen.Gen.GenKcl(kclSchema, "values", fixedJSONSchema, &kclgen.GenKclOptions{
		Mode:                  kclgen.ModeJSONSchema,
		CastingOption:         kclgen.OriginalName,
		UseIntegersForNumbers: true,
		RemoveDefaults:        removeDefaults,
	})
	if err != nil {
		return nil, fmt.Errorf("generate kcl schema: %w", err)
	}

	return kclSchema.Bytes(), nil
}

// ConvertToKCLCompatibleJSONSchema converts a JSON schema to a JSON schema that
// is compatible with KCL schema generation (i.e. removing unsupported fields).
func ConvertToKCLCompatibleJSONSchema(jsonSchemaData []byte) ([]byte, error) {
	s, err := unmarshalSchema(jsonSchemaData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON Schema: %w", err)
	}

	// Remove the ID to keep KCL schema naming consistent.
	s.ID = ""

	// Merge into an empty schema, which results in a flattened schema that is
	// compatible with KCL schema generation.
	ms := mergeSchemas(&gojsonschema.Schema{}, s, true)

	fixedJSONSchema, err := marshalSchema(ms)
	if err != nil {
		return nil, fmt.Errorf("convert schema to JSON: %w", err)
	}

	fixedJSONSchema, err = fixGenSchemas(fixedJSONSchema)
	if err != nil {
		return nil, fmt.Errorf("fix schemas for generation: %w", err)
	}

	return fixedJSONSchema, nil
}

// fixGenSchemas rewrites constructs that KCL schema generation mishandles:
//
//   - Boolean sub-schemas (the JSON Schema true and false forms) become empty
//     object schemas in positions where the KCL gen tool cannot handle them.
//     Empty schemas marshal to true, but the gen tool only accepts booleans
//     for additionalProperties; a boolean property schema mangles the
//     property name and a boolean items schema is a hard error.
//
//   - Array schemas without an items schema gain an empty one, so the gen
//     tool emits the [any] list type instead of any.
func fixGenSchemas(jsonSchemaData []byte) ([]byte, error) {
	var doc any

	err := json.Unmarshal(jsonSchemaData, &doc)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON Schema: %w", err)
	}

	schema, ok := doc.(map[string]any)
	if !ok {
		return jsonSchemaData, nil
	}

	fixGenSubSchemas(schema)

	fixedData, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal JSON Schema: %w", err)
	}

	return fixedData, nil
}

// fixGenSubSchemas recursively replaces boolean schemas with empty object
// schemas in all sub-schema positions except additionalProperties, which the
// KCL gen tool accepts in boolean form, and gives array schemas an items
// schema when they lack one. Items are only added for the single "array"
// type; the gen tool renders type unions like ["array", "null"] better
// without one.
func fixGenSubSchemas(schema map[string]any) {
	if schema["type"] == "array" {
		if _, ok := schema["items"]; !ok {
			schema["items"] = map[string]any{}
		}
	}

	for _, key := range [...]string{"properties", "patternProperties", "definitions", "$defs"} {
		properties, ok := schema[key].(map[string]any)
		if !ok {
			continue
		}

		for name, value := range properties {
			switch sub := value.(type) {
			case bool:
				properties[name] = map[string]any{}
			case map[string]any:
				fixGenSubSchemas(sub)
			}
		}
	}

	for _, key := range [...]string{"items", "not", "if", "then", "else"} {
		switch sub := schema[key].(type) {
		case bool:
			schema[key] = map[string]any{}
		case map[string]any:
			fixGenSubSchemas(sub)
		}
	}

	if sub, ok := schema["additionalProperties"].(map[string]any); ok {
		fixGenSubSchemas(sub)
	}

	for _, key := range [...]string{"allOf", "anyOf", "oneOf"} {
		subSchemas, ok := schema[key].([]any)
		if !ok {
			continue
		}

		for i, value := range subSchemas {
			switch sub := value.(type) {
			case bool:
				subSchemas[i] = map[string]any{}
			case map[string]any:
				fixGenSubSchemas(sub)
			}
		}
	}
}

// mergeSchemas merges src into dest and returns the result, flattening
// compositors (allOf, anyOf, oneOf, if/then/else, not) into a single schema
// and dropping keywords that KCL schema generation cannot handle. Merging a
// schema into an empty destination produces its flattened, KCL-compatible
// form. Required keys survive only when present on both sides, so flattening
// drops them; partial values overlays must stay valid against the generated
// KCL schema.
func mergeSchemas(dest, src *gojsonschema.Schema, setDefaults bool) *gojsonschema.Schema {
	if dest == nil {
		return mergeSchemas(&gojsonschema.Schema{}, src, setDefaults)
	}

	if src == nil {
		return mergeSchemas(&gojsonschema.Schema{}, dest, setDefaults)
	}

	if setDefaults {
		dest.Default = src.Default
	}

	// Merge 'type' by unioning all observed types, so that compositor branches
	// which disagree (e.g. an anyOf with a null branch and an object branch)
	// flatten to a schema that accepts both.
	mergeSchemaTypes(dest, src)

	// Resolve simple fields by favoring the fields from 'src' if they're provided.
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

	if src.ID != "" {
		dest.ID = src.ID
	}

	// Merge 'enum' field, dropping duplicates while preserving order.
	dest.Enum = dedupeEnum(append(dest.Enum, src.Enum...))

	dest.Required = intersectStrings(dest.Required, src.Required)

	// Recursive calls for nested structures.
	if src.Properties != nil {
		if dest.Properties == nil {
			dest.Properties = make(map[string]*gojsonschema.Schema)
		}

		for _, k := range slices.Sorted(maps.Keys(src.Properties)) {
			dest.Properties[k] = mergeSchemas(dest.Properties[k], src.Properties[k], setDefaults)
		}
	}

	if src.AdditionalProperties != nil {
		err := mergeSchemaAdditionalProperties(dest, src, setDefaults)
		if err != nil {
			dest.AdditionalProperties = &gojsonschema.Schema{}
		}
	}

	// Merge 'items' if they exist (assuming they're not arrays).
	if src.Items != nil {
		dest.Items = mergeSchemas(dest.Items, src.Items, setDefaults)
	}

	// Flatten the compositors into a single schema, dropping the keywords once
	// their branches have been merged in.
	var combined *gojsonschema.Schema

	for _, s := range append(dest.AllOf, src.AllOf...) {
		combined = mergeSchemas(combined, s, setDefaults)
	}

	for _, s := range append(dest.AnyOf, src.AnyOf...) {
		combined = mergeSchemas(combined, s, setDefaults)
	}

	for _, s := range append(dest.OneOf, src.OneOf...) {
		combined = mergeSchemas(combined, s, setDefaults)
	}

	dest.AllOf, dest.AnyOf, dest.OneOf = nil, nil, nil

	if combined != nil {
		dest = mergeSchemas(dest, combined, setDefaults)
	}

	if src.If != nil {
		dest = mergeSchemas(dest, src.If, setDefaults)
	}

	if src.Else != nil {
		dest = mergeSchemas(dest, src.Else, setDefaults)
	}

	if src.Then != nil {
		dest = mergeSchemas(dest, src.Then, setDefaults)
	}

	if src.Not != nil {
		dest = mergeSchemas(dest, src.Not, setDefaults)
	}

	return dest
}

// mergeSchemaTypes unions the type constraints of src into dest. The union is
// sorted and deduplicated, and stored on [gojsonschema.Schema.Type] when it
// contains a single type, or [gojsonschema.Schema.Types] otherwise.
func mergeSchemaTypes(dest, src *gojsonschema.Schema) {
	srcTypes := schemaTypeList(src)
	if len(srcTypes) == 0 {
		return
	}

	destTypes := schemaTypeList(dest)

	switch {
	case len(destTypes) == 0:
		setSchemaTypes(dest, srcTypes)

	case !slices.Equal(destTypes, srcTypes):
		merged := slices.Concat(destTypes, srcTypes)
		slices.Sort(merged)
		setSchemaTypes(dest, slices.Compact(merged))
	}
}

// schemaTypeList returns the schema's type constraint as a list, regardless of
// whether it is stored on Type or Types.
func schemaTypeList(s *gojsonschema.Schema) []string {
	if s.Type != "" {
		return []string{s.Type}
	}

	return slices.Clone(s.Types)
}

// setSchemaTypes stores a type list on the schema, using Type for a single
// type and Types otherwise.
func setSchemaTypes(s *gojsonschema.Schema, types []string) {
	if len(types) == 1 {
		s.Type = types[0]
		s.Types = nil

		return
	}

	s.Type = ""
	s.Types = types
}

// dedupeEnum returns enum with duplicate values removed, preserving the order
// of first appearance. Values are compared by their JSON encoding so that
// non-comparable values (objects, arrays) do not panic, unlike slices.Compact
// on a []any, which also only collapses adjacent duplicates.
func dedupeEnum(enum []any) []any {
	if len(enum) == 0 {
		return enum
	}

	seen := make(map[string]bool, len(enum))
	out := make([]any, 0, len(enum))

	for _, v := range enum {
		key, err := json.Marshal(v)
		if err != nil {
			// Keep values that cannot be encoded rather than dropping them.
			out = append(out, v)

			continue
		}

		if seen[string(key)] {
			continue
		}

		seen[string(key)] = true

		out = append(out, v)
	}

	return out
}

func intersectStrings(a, b []string) []string {
	intersection := []string{}

	for _, x := range a {
		if slices.Contains(b, x) {
			intersection = append(intersection, x)
		}
	}

	return intersection
}

func mergeSchemaAdditionalProperties(dest, src *gojsonschema.Schema, setDefaults bool) error {
	if isBoolSchema(src.AdditionalProperties) {
		dest.AdditionalProperties = src.AdditionalProperties

		return nil
	}

	subSchema := mergeSchemas(dest.AdditionalProperties, src.AdditionalProperties, setDefaults)

	err := validateSchema(subSchema)
	if err != nil {
		return err
	}

	dest.AdditionalProperties = subSchema

	return nil
}

// isBoolSchema reports whether s marshals to one of the boolean JSON Schema
// forms: true (the empty schema) or false (its negation).
func isBoolSchema(s *gojsonschema.Schema) bool {
	if s == nil {
		return false
	}

	data, err := json.Marshal(s)
	if err != nil {
		return false
	}

	return bytes.Equal(data, []byte("true")) || bytes.Equal(data, []byte("false"))
}
