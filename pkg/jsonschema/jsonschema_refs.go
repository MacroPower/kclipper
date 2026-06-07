// Copyright (c) 2023 dadav, Licensed under the MIT License.
// Modifications Copyright (c) 2024-2025 Jacob Colvin
// Licensed under the Apache License, Version 2.0.

package jsonschema

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	gojsonschema "github.com/google/jsonschema-go/jsonschema"
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
func handleSchemaRefs(schema *gojsonschema.Schema, basePath string) error {
	return resolveSchemaRefs(schema, basePath, map[string]bool{})
}

// resolveSchemaRefs implements [handleSchemaRefs]. The resolving set tracks
// the stack of file paths currently being resolved, so that mutually
// referencing files produce an error instead of infinite recursion. Paths are
// removed once resolved, keeping repeated (diamond) references to the same
// file valid.
func resolveSchemaRefs(schema *gojsonschema.Schema, basePath string, resolving map[string]bool) error {
	if schema == nil {
		return nil
	}

	// AdditionalProperties is special: an unresolvable reference fails open
	// (becomes the permissive empty schema) instead of aborting the schema.
	if schema.AdditionalProperties != nil {
		err := resolveSchemaRefs(schema.AdditionalProperties, basePath, resolving)
		if err != nil {
			schema.AdditionalProperties = &gojsonschema.Schema{}
		}
	}

	// Recurse into every other sub-schema position. This mirrors [walkSchema]
	// so that no $ref-bearing field is missed (e.g. $defs, definitions,
	// prefixItems, contains, propertyNames).
	for _, subSchema := range [...]*gojsonschema.Schema{
		schema.Items, schema.AdditionalItems, schema.Contains, schema.UnevaluatedItems,
		schema.PropertyNames, schema.UnevaluatedProperties,
		schema.Not, schema.If, schema.Then, schema.Else, schema.ContentSchema,
	} {
		err := resolveSchemaRefs(subSchema, basePath, resolving)
		if err != nil {
			return err
		}
	}

	for _, subSchemas := range [...][]*gojsonschema.Schema{
		schema.PrefixItems, schema.ItemsArray, schema.AllOf, schema.AnyOf, schema.OneOf,
	} {
		for _, subSchema := range subSchemas {
			err := resolveSchemaRefs(subSchema, basePath, resolving)
			if err != nil {
				return err
			}
		}
	}

	for _, subSchemas := range [...]map[string]*gojsonschema.Schema{
		schema.Defs, schema.Definitions, schema.DependencySchemas,
		schema.Properties, schema.PatternProperties, schema.DependentSchemas,
	} {
		for _, subSchema := range subSchemas {
			err := resolveSchemaRefs(subSchema, basePath, resolving)
			if err != nil {
				return err
			}
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

		err = resolveFilePath(schema, relFilePath, jsPointer, resolving)
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

	return validateSchema(schema)
}

func resolveLocalRef(schema *gojsonschema.Schema, jsonSchemaPointer string) (*gojsonschema.Schema, error) {
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

func resolveFilePath(schema *gojsonschema.Schema, relPath, jsonSchemaPointer string, resolving map[string]bool) error {
	cleanPath := path.Clean(relPath)
	if resolving[cleanPath] {
		return fmt.Errorf("circular reference to %q", cleanPath)
	}

	resolving[cleanPath] = true
	defer delete(resolving, cleanPath)

	//nolint:gosec // G304 not relevant for client-side generation.
	byteValue, err := os.ReadFile(relPath)
	if err != nil {
		return fmt.Errorf("read file %q: %w", relPath, err)
	}

	relSchema, err := unmarshalSchemaRef(byteValue, jsonSchemaPointer)
	if err != nil {
		return fmt.Errorf("unmarshal schema: %w", err)
	}

	err = resolveSchemaRefs(relSchema, path.Dir(relPath), resolving)
	if err != nil {
		return err
	}

	*schema = *relSchema

	return nil
}

func unmarshalSchemaRef(data []byte, jsonSchemaPointer string) (*gojsonschema.Schema, error) {
	if jsonSchemaPointer == "" {
		relSchema, err := unmarshalSchema(data)
		if err != nil {
			return nil, err
		}

		err = validateSchema(relSchema)
		if err != nil {
			return nil, err
		}

		return relSchema, nil
	}

	var obj any

	err := json.Unmarshal(data, &obj)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON Schema: %w", err)
	}

	jsonPointerResultRaw, err := evalJSONPointer(obj, jsonSchemaPointer)
	if err != nil {
		return nil, fmt.Errorf("resolve JSON pointer: %w", err)
	}

	jsonPointerResultMarshaled, err := json.Marshal(jsonPointerResultRaw)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON pointer result: %w", err)
	}

	relSchema := &gojsonschema.Schema{}

	err = json.Unmarshal(jsonPointerResultMarshaled, relSchema)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON pointer result: %w", err)
	}

	err = validateSchema(relSchema)
	if err != nil {
		return nil, err
	}

	return relSchema, nil
}

// evalJSONPointer evaluates an RFC 6901 JSON pointer against a decoded JSON
// document and returns the referenced value.
func evalJSONPointer(doc any, pointer string) (any, error) {
	if pointer == "" {
		return doc, nil
	}

	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("invalid JSON pointer %q", pointer)
	}

	current := doc

	for segment := range strings.SplitSeq(pointer[1:], "/") {
		segment = strings.ReplaceAll(segment, "~1", "/")
		segment = strings.ReplaceAll(segment, "~0", "~")

		switch value := current.(type) {
		case map[string]any:
			child, ok := value[segment]
			if !ok {
				return nil, fmt.Errorf("JSON pointer segment %q not found", segment)
			}

			current = child

		case []any:
			idx, err := strconv.Atoi(segment)
			if err != nil {
				return nil, fmt.Errorf("JSON pointer segment %q is not an array index: %w", segment, err)
			}

			if idx < 0 || idx >= len(value) {
				return nil, fmt.Errorf("JSON pointer segment %q is out of range", segment)
			}

			current = value[idx]

		default:
			return nil, fmt.Errorf("JSON pointer segment %q references a non-container value", segment)
		}
	}

	return current, nil
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
