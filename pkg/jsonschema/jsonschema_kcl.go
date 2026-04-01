package jsonschema

import (
	"bytes"
	"fmt"
	"slices"

	gjs "github.com/google/jsonschema-go/jsonschema"

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
	hs, err := unmarshalSchema(jsonSchemaData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON Schema: %w", err)
	}

	// Remove the ID to keep KCL schema naming consistent.
	hs.ID = ""

	// For now, flatten into an empty schema as that will result in a schema
	// that is compatible with KCL schema generation.
	mhs := &gjs.Schema{}
	mhs = flattenSchemaForKCL(mhs, hs)

	fixedJSONSchema, err := marshalSchemaIndent(mhs)
	if err != nil {
		return nil, fmt.Errorf("convert schema to JSON: %w", err)
	}

	return fixedJSONSchema, nil
}

// flattenSchemaForKCL recursively flattens a JSON Schema into a KCL-compatible
// form by inlining composition (allOf/anyOf/oneOf) and conditional
// (if/then/else/not) keywords, and copying scalar fields from src into dest.
func flattenSchemaForKCL(dest, src *gjs.Schema) *gjs.Schema {
	if dest == nil {
		return flattenSchemaForKCL(&gjs.Schema{}, src)
	}

	if src == nil {
		return flattenSchemaForKCL(&gjs.Schema{}, dest)
	}

	copyScalarFields(dest, src)
	flattenProperties(dest, src)
	flattenAdditionalProperties(dest, src)

	if src.Items != nil {
		dest.Items = flattenSchemaForKCL(dest.Items, src.Items)
	}

	dest = flattenCompositionKeywords(dest, src)
	dest = flattenConditionalKeywords(dest, src)

	return dest
}

// copyScalarFields copies non-zero scalar and metadata fields, default values,
// and enum values from src into dest. It also clears Required since KCL does
// not support the required keyword.
func copyScalarFields(dest, src *gjs.Schema) {
	dest.Default = src.Default

	if src.Type != "" || src.Types != nil {
		dest.Type = src.Type
		dest.Types = src.Types
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

	if src.ID != "" {
		dest.ID = src.ID
	}

	dest.Enum = slices.Compact(append(dest.Enum, src.Enum...))

	// KCL does not support the required keyword, so clear it.
	dest.Required = nil
}

// flattenProperties recursively flattens each property sub-schema from src
// into dest.
func flattenProperties(dest, src *gjs.Schema) {
	if src.Properties == nil {
		return
	}

	if dest.Properties == nil {
		dest.Properties = make(map[string]*gjs.Schema)
	}

	propKeys := []string{}
	for k := range src.Properties {
		propKeys = append(propKeys, k)
	}

	slices.Sort(propKeys)

	for _, k := range propKeys {
		if destPropSchema, exists := dest.Properties[k]; exists {
			dest.Properties[k] = flattenSchemaForKCL(destPropSchema, src.Properties[k])
		} else {
			dest.Properties[k] = flattenSchemaForKCL(&gjs.Schema{}, src.Properties[k])
		}
	}
}

// flattenAdditionalProperties handles the additionalProperties field by either
// copying a boolean schema directly or recursively flattening the sub-schema.
func flattenAdditionalProperties(dest, src *gjs.Schema) {
	if src.AdditionalProperties == nil {
		return
	}

	if isBooleanSchema(src.AdditionalProperties) {
		dest.AdditionalProperties = src.AdditionalProperties
		return
	}

	destAP := dest.AdditionalProperties
	if destAP == nil {
		destAP = &gjs.Schema{}
	}

	dest.AdditionalProperties = flattenSchemaForKCL(destAP, src.AdditionalProperties)
}

// flattenCompositionKeywords inlines allOf, anyOf, and oneOf sub-schemas from
// src into dest and returns the resulting schema.
func flattenCompositionKeywords(dest, src *gjs.Schema) *gjs.Schema {
	var items *gjs.Schema
	for _, s := range append(dest.AllOf, src.AllOf...) {
		items = flattenSchemaForKCL(items, s)
		dest.AllOf = nil
	}

	for _, s := range append(dest.AnyOf, src.AnyOf...) {
		items = flattenSchemaForKCL(items, s)
		dest.AnyOf = nil
	}

	for _, s := range append(dest.OneOf, src.OneOf...) {
		items = flattenSchemaForKCL(items, s)
		dest.OneOf = nil
	}

	if items != nil {
		dest = flattenSchemaForKCL(dest, items)
	}

	return dest
}

// flattenConditionalKeywords inlines if, then, else, and not sub-schemas from
// src into dest and returns the resulting schema.
func flattenConditionalKeywords(dest, src *gjs.Schema) *gjs.Schema {
	if src.If != nil {
		dest = flattenSchemaForKCL(dest, src.If)
	}

	if src.Else != nil {
		dest = flattenSchemaForKCL(dest, src.Else)
	}

	if src.Then != nil {
		dest = flattenSchemaForKCL(dest, src.Then)
	}

	if src.Not != nil {
		dest = flattenSchemaForKCL(dest, src.Not)
	}

	return dest
}
