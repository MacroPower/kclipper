// Copyright (c) 2023 dadav, Licensed under the MIT License.
// Modifications Copyright (c) 2024-2025 Jacob Colvin
// Licensed under the Apache License, Version 2.0.

package jsonschema

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	gjs "github.com/google/jsonschema-go/jsonschema"
)

// handleSchemaRefs processes and resolves JSON Schema references ($ref) within a schema.
// It handles both direct schema references and references within patternProperties.
// For each reference:
// - If it's a relative file path, it attempts to load and parse the referenced schema.
// - If it includes a JSON pointer (#/path/to/schema), it extracts the specific schema section.
// - The resolved schema replaces the original reference.
//
// Parameters:
//   - schema: Pointer to the Schema object containing the references to resolve.
//   - basePath: Path to the current file, used for resolving relative paths.
func handleSchemaRefs(schema *gjs.Schema, basePath string) error {
	// Handle $ref in PatternProperties.
	if schema.PatternProperties != nil {
		for pattern, subSchema := range schema.PatternProperties {
			err := handleSchemaRefs(subSchema, basePath)
			if err != nil {
				return err
			}

			// Update the original schema in the map.
			schema.PatternProperties[pattern] = subSchema
		}
	}

	// Handle $ref in Properties.
	if schema.Properties != nil {
		for property, subSchema := range schema.Properties {
			err := handleSchemaRefs(subSchema, basePath)
			if err != nil {
				return err
			}

			// Update the original schema in the map.
			schema.Properties[property] = subSchema
		}
	}

	// Handle $ref in AdditionalProperties.
	err := derefAdditionalProperties(schema, basePath)
	if err != nil {
		schema.AdditionalProperties = &gjs.Schema{}
	}

	// Handle $ref in Items.
	if schema.Items != nil {
		subSchema := schema.Items
		err := handleSchemaRefs(subSchema, basePath)
		if err != nil {
			return err
		}

		// Update the original schema.
		schema.Items = subSchema
	}

	// Handle $ref in AllOf.
	if schema.AllOf != nil {
		for i, subSchema := range schema.AllOf {
			err := handleSchemaRefs(subSchema, basePath)
			if err != nil {
				return err
			}

			// Update the original schema in the slice.
			schema.AllOf[i] = subSchema
		}
	}

	// Handle $ref in AnyOf.
	if schema.AnyOf != nil {
		for i, subSchema := range schema.AnyOf {
			err := handleSchemaRefs(subSchema, basePath)
			if err != nil {
				return err
			}

			// Update the original schema in the slice.
			schema.AnyOf[i] = subSchema
		}
	}

	// Handle $ref in OneOf.
	if schema.OneOf != nil {
		for i, subSchema := range schema.OneOf {
			err := handleSchemaRefs(subSchema, basePath)
			if err != nil {
				return err
			}

			// Update the original schema in the slice.
			schema.OneOf[i] = subSchema
		}
	}

	// Handle $ref in If.
	if schema.If != nil {
		err := handleSchemaRefs(schema.If, basePath)
		if err != nil {
			return err
		}
	}

	// Handle $ref in Then.
	if schema.Then != nil {
		err := handleSchemaRefs(schema.Then, basePath)
		if err != nil {
			return err
		}
	}

	// Handle $ref in Else.
	if schema.Else != nil {
		err := handleSchemaRefs(schema.Else, basePath)
		if err != nil {
			return err
		}
	}

	// Handle $ref in Not.
	if schema.Not != nil {
		err := handleSchemaRefs(schema.Not, basePath)
		if err != nil {
			return err
		}
	}

	// Handle main schema $ref.
	if schema.Ref == "" {
		return nil
	}

	jsFilePath, jsPointer, found := strings.Cut(schema.Ref, "#")
	if !found {
		return fmt.Errorf("invalid $ref value %q", schema.Ref)
	}

	if jsFilePath != "" {
		relFilePath, err := isRelativeFile(basePath, jsFilePath)
		if err != nil {
			return fmt.Errorf("invalid $ref value %q: %w", schema.Ref, err)
		}

		err = resolveFilePath(schema, relFilePath, jsPointer)
		if err != nil {
			return fmt.Errorf("invalid $ref value %q: %w", schema.Ref, err)
		}
	}

	if jsFilePath == "" && jsPointer != "" {
		relSchema, err := resolveLocalRef(schema, jsPointer)

		// Sometimes this will error due to a partial import, where references
		// don't exist in the current document.
		if err != nil {
			schema.Ref = ""
		} else {
			*schema = *relSchema
		}
	}

	return nil
}

func resolveLocalRef(schema *gjs.Schema, jsonSchemaPointer string) (*gjs.Schema, error) {
	relData, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema for json pointer: %w", err)
	}

	relSchema, err := unmarshalSchemaRef(relData, jsonSchemaPointer)
	if err != nil {
		return nil, fmt.Errorf("unmarshal schema for json pointer: %w", err)
	}

	return relSchema, nil
}

func resolveFilePath(schema *gjs.Schema, relPath, jsonSchemaPointer string) error {
	//nolint:gosec // G304 not relevant for client-side generation.
	byteValue, err := os.ReadFile(relPath)
	if err != nil {
		return fmt.Errorf("read file %q: %w", relPath, err)
	}

	relSchema, err := unmarshalSchemaRef(byteValue, jsonSchemaPointer)
	if err != nil {
		return fmt.Errorf("unmarshal schema: %w", err)
	}

	err = handleSchemaRefs(relSchema, path.Dir(relPath))
	if err != nil {
		return err
	}

	*schema = *relSchema

	return nil
}

func unmarshalSchemaRef(data []byte, jsonSchemaPointer string) (*gjs.Schema, error) {
	relSchema := &gjs.Schema{}

	if jsonSchemaPointer == "" {
		err := json.Unmarshal(data, relSchema)
		if err != nil {
			return nil, fmt.Errorf("unmarshal JSON Schema: %w", err)
		}

		return relSchema, nil
	}

	var obj any

	err := json.Unmarshal(data, &obj)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON Schema: %w", err)
	}

	jsonPointerResultRaw, err := resolveJSONPointer(obj, jsonSchemaPointer)
	if err != nil {
		return nil, fmt.Errorf("resolve JSON pointer: %w", err)
	}

	jsonPointerResultMarshaled, err := json.Marshal(jsonPointerResultRaw)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON pointer result: %w", err)
	}

	err = json.Unmarshal(jsonPointerResultMarshaled, relSchema)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON pointer result: %w", err)
	}

	return relSchema, nil
}

func derefAdditionalProperties(schema *gjs.Schema, basePath string) error {
	if schema.AdditionalProperties == nil || isBooleanSchema(schema.AdditionalProperties) {
		return nil
	}

	err := handleSchemaRefs(schema.AdditionalProperties, basePath)
	if err != nil {
		return fmt.Errorf("handle schema refs in additional properties: %w", err)
	}

	return nil
}

// isBooleanSchema reports whether s is a boolean schema (true or false).
// In [gjs.Schema], true is represented as an empty schema and false as
// a schema containing only {Not: &Schema{}}.
func isBooleanSchema(s *gjs.Schema) bool {
	if s == nil {
		return false
	}

	data, err := json.Marshal(s)
	if err != nil {
		return false
	}

	return string(data) == "true" || string(data) == "false"
}

// isRelativeFile checks if the given string is a relative path to a file.
func isRelativeFile(root, relPath string) (string, error) {
	if !path.IsAbs(relPath) {
		rp := path.Join(root, relPath)
		_, err := os.Stat(rp)
		if err != nil {
			return "", fmt.Errorf("stat file %q: %w", rp, err)
		}

		return rp, nil
	}

	return "", fmt.Errorf("%q is not a relative path", relPath)
}
