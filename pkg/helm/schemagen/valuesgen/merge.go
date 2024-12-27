package valuesgen

import (
	helmschema "github.com/dadav/helm-schema/pkg/schema"
)

func Merge(dest, src *helmschema.Schema, setDefaults bool) *helmschema.Schema {
	if dest == nil {
		return src
	}
	if src == nil {
		return dest
	}

	if setDefaults {
		dest.Default = src.Default
	}

	// Resolve simple fields by favoring the fields from 'src' if they're provided
	if !src.Type.IsEmpty() {
		dest.Type = src.Type
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
	dest.AnyOf = append(dest.AnyOf, src.AnyOf...)
	dest.OneOf = append(dest.OneOf, src.OneOf...)
	dest.AllOf = append(dest.AllOf, src.AllOf...)

	// Recursive calls for nested structures
	if src.Properties != nil {
		if dest.Properties == nil {
			dest.Properties = make(map[string]*helmschema.Schema)
		}
		for propName, srcPropSchema := range src.Properties {
			if destPropSchema, exists := dest.Properties[propName]; exists {
				dest.Properties[propName] = Merge(destPropSchema, srcPropSchema, setDefaults)
			} else {
				dest.Properties[propName] = srcPropSchema
			}
		}
	}

	// Merge 'items' if they exist (assuming they're not arrays)
	if src.Items != nil {
		dest.Items = Merge(dest.Items, src.Items, setDefaults)
	}

	var items *helmschema.Schema
	for _, s := range dest.AllOf {
		items = Merge(items, s, setDefaults)
		dest.AllOf = nil
	}
	for _, s := range dest.AnyOf {
		items = Merge(items, s, setDefaults)
		dest.AnyOf = nil
	}
	for _, s := range dest.OneOf {
		items = Merge(items, s, setDefaults)
		dest.OneOf = nil
	}
	if items != nil {
		dest.Items = Merge(dest.Items, items, setDefaults)
	}

	if src.If != nil {
		dest.If = Merge(dest.If, src.If, setDefaults)
	}
	if src.Else != nil {
		dest.Else = Merge(dest.Else, src.Else, setDefaults)
	}
	if src.Then != nil {
		dest.Then = Merge(dest.Then, src.Then, setDefaults)
	}
	if src.Not != nil {
		dest.Not = Merge(dest.Not, src.Not, setDefaults)
	}

	return dest
}
