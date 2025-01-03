// Copyright (c) 2023 dadav, Licensed under the MIT License.
// Modifications Copyright (c) 2024-2025 Jacob Colvin
// Licensed under the Apache License, Version 2.0

package jsonschema

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/dadav/go-jsonpointer"
	helmschema "github.com/dadav/helm-schema/pkg/schema"
	"gopkg.in/yaml.v3"
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
func handleSchemaRefs(schema *helmschema.Schema, basePath string) error {
	// Handle $ref in pattern properties
	if schema.PatternProperties != nil {
		for pattern, subSchema := range schema.PatternProperties {
			if err := handleSchemaRefs(subSchema, basePath); err != nil {
				return err
			}
			schema.PatternProperties[pattern] = subSchema // Update the original schema in the map
		}
	}

	// Handle $ref in properties
	if schema.Properties != nil {
		for property, subSchema := range schema.Properties {
			if err := handleSchemaRefs(subSchema, basePath); err != nil {
				return err
			}
			schema.Properties[property] = subSchema // Update the original schema in the map
		}
	}

	// Handle $ref in additional properties
	if err := derefAdditionalProperties(schema, basePath); err != nil {
		schema.AdditionalProperties = true
	}

	// Handle $ref in items
	if schema.Items != nil {
		subSchema := schema.Items
		if err := handleSchemaRefs(subSchema, basePath); err != nil {
			return err
		}
		schema.Items = subSchema // Update the original schema
	}

	// Handle $ref in allOf
	if schema.AllOf != nil {
		for i, subSchema := range schema.AllOf {
			if err := handleSchemaRefs(subSchema, basePath); err != nil {
				return err
			}
			schema.AllOf[i] = subSchema // Update the original schema in the slice
		}
	}

	// Handle $ref in anyOf
	if schema.AnyOf != nil {
		for i, subSchema := range schema.AnyOf {
			if err := handleSchemaRefs(subSchema, basePath); err != nil {
				return err
			}
			schema.AnyOf[i] = subSchema // Update the original schema in the slice
		}
	}

	// Handle $ref in oneOf
	if schema.OneOf != nil {
		for i, subSchema := range schema.OneOf {
			if err := handleSchemaRefs(subSchema, basePath); err != nil {
				return err
			}
			schema.OneOf[i] = subSchema // Update the original schema in the slice
		}
	}

	// Handle main schema $ref
	if schema.Ref == "" {
		return nil
	}

	jsFilePath, jsPointer, found := strings.Cut(schema.Ref, "#")
	if !found {
		return fmt.Errorf("invalid $ref value '%s'", schema.Ref)
	}

	if jsFilePath != "" {
		relFilePath, err := isRelativeFile(basePath, jsFilePath)
		if err != nil {
			return fmt.Errorf("invalid $ref value '%s': %w", schema.Ref, err)
		}
		if err := resolveFilePath(schema, relFilePath, jsPointer); err != nil {
			return fmt.Errorf("invalid $ref value '%s': %w", schema.Ref, err)
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
			schema.HasData = true
		}
	}

	if err := schema.Validate(); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	return nil
}

func resolveLocalRef(schema *helmschema.Schema, jsonSchemaPointer string) (*helmschema.Schema, error) {
	relData, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema for json pointer: %w", err)
	}
	relSchema, err := unmarshalSchemaRef(relData, jsonSchemaPointer)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema for json pointer: %w", err)
	}
	return relSchema, nil
}

func resolveFilePath(schema *helmschema.Schema, relPath, jsonSchemaPointer string) error {
	file, err := os.Open(relPath)
	if err != nil {
		return fmt.Errorf("error opening file '%s': %w", relPath, err)
	}
	defer file.Close()
	byteValue, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error reading file '%s': %w", relPath, err)
	}

	relSchema, err := unmarshalSchemaRef(byteValue, jsonSchemaPointer)
	if err != nil {
		return fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	if err := handleSchemaRefs(relSchema, path.Dir(relPath)); err != nil {
		return err
	}

	*schema = *relSchema
	schema.HasData = true

	return nil
}

func unmarshalSchemaRef(data []byte, jsonSchemaPointer string) (*helmschema.Schema, error) {
	relSchema := &helmschema.Schema{}

	if jsonSchemaPointer == "" {
		err := json.Unmarshal(data, &relSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON Schema: %w", err)
		}
		if err := relSchema.Validate(); err != nil {
			return nil, fmt.Errorf("invalid schema: %w", err)
		}
		return relSchema, nil
	}

	var obj interface{}
	err := json.Unmarshal(data, &obj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON Schema: %w", err)
	}
	jsonPointerResultRaw, err := jsonpointer.Get(obj, jsonSchemaPointer)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve JSON pointer: %w", err)
	}
	jsonPointerResultMarshaled, err := json.Marshal(jsonPointerResultRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON pointer result: %w", err)
	}
	err = json.Unmarshal(jsonPointerResultMarshaled, relSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON pointer result: %w", err)
	}

	if err := relSchema.Validate(); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	return relSchema, nil
}

func derefAdditionalProperties(schema *helmschema.Schema, basePath string) error {
	if schema.AdditionalProperties == nil || schema.AdditionalProperties == true || schema.AdditionalProperties == false {
		return nil
	}

	apData, err := json.Marshal(schema.AdditionalProperties)
	if err != nil {
		return err //nolint:wrapcheck
	}

	subSchema := &helmschema.Schema{}
	var jsonNode yaml.Node
	if err := yaml.Unmarshal(apData, &jsonNode); err != nil {
		return err //nolint:wrapcheck
	}
	if err := subSchema.UnmarshalYAML(&jsonNode); err != nil {
		return err //nolint:wrapcheck
	}
	if err := handleSchemaRefs(subSchema, basePath); err != nil {
		return err
	}
	if err := subSchema.Validate(); err != nil {
		return err //nolint:wrapcheck
	}

	subSchema.Required = helmschema.BoolOrArrayOfString{}
	schema.AdditionalProperties = subSchema

	return nil
}

// isRelativeFile checks if the given string is a relative path to a file.
func isRelativeFile(root, relPath string) (string, error) {
	if !path.IsAbs(relPath) {
		rp := path.Join(root, relPath)
		_, err := os.Stat(rp)
		if err != nil {
			return "", fmt.Errorf("failed to describe file '%s': %w", rp, err)
		}
		return rp, nil
	}
	return "", fmt.Errorf("'%s' is not a relative path", relPath)
}
