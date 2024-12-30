// Copyright (c) 2023 dadav, Licensed under the MIT License.
// Modifications Copyright (c) 2024 MacroPower, Licensed under the Apache License, Version 2.0.

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
	"github.com/hashicorp/go-multierror"
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
		var relSchema helmschema.Schema
		file, err := os.Open(relFilePath)
		if err != nil {
			return fmt.Errorf("error opening file '%s' for $ref '%s': %w", relFilePath, schema.Ref, err)
		}
		defer file.Close()
		byteValue, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("error reading file '%s' for $ref '%s': %w", relFilePath, schema.Ref, err)
		}

		if jsPointer != "" {
			// Found json-pointer
			var obj interface{}
			var merr error
			err := json.Unmarshal(byteValue, &obj)
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			jsonPointerResultRaw, err := jsonpointer.Get(obj, jsPointer)
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			jsonPointerResultMarshaled, err := json.Marshal(jsonPointerResultRaw)
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			err = json.Unmarshal(jsonPointerResultMarshaled, &relSchema)
			if err != nil {
				merr = multierror.Append(merr, err)
				return fmt.Errorf("failed to resolve JSON pointer in $ref '%s': %w", schema.Ref, merr)
			}
		} else {
			// No json-pointer
			err = json.Unmarshal(byteValue, &relSchema)
			if err != nil {
				return fmt.Errorf("failed to unmarshal JSON Schema for $ref '%s': %w", schema.Ref, err)
			}
		}

		if err := relSchema.Validate(); err != nil {
			return fmt.Errorf("encountered invalid schema while resolving $ref '%s': %w", schema.Ref, err)
		}

		if err := handleSchemaRefs(&relSchema, path.Dir(relFilePath)); err != nil {
			return err
		}

		*schema = relSchema
		schema.HasData = true
	}

	if jsFilePath == "" && jsPointer != "" {
		var merr error
		relData, err := json.Marshal(schema)
		if err != nil {
			merr = multierror.Append(merr, err)
		}
		jsonPointerResultRaw, err := jsonpointer.Get(relData, jsPointer)
		if err != nil {
			merr = multierror.Append(merr, err)
		}
		jsonPointerResultMarshaled, err := json.Marshal(jsonPointerResultRaw)
		if err != nil {
			merr = multierror.Append(merr, err)
		}
		var relSchema helmschema.Schema
		err = json.Unmarshal(jsonPointerResultMarshaled, &relSchema)
		if err != nil {
			merr = multierror.Append(merr, err)
		}
		if err := relSchema.Validate(); err != nil {
			merr = multierror.Append(merr, err)
		}

		// Sometimes this will error due to a partial import, where references
		// don't exist in the current document.
		if merr != nil {
			schema.Ref = ""
		} else {
			*schema = relSchema
			schema.HasData = true
		}
	}

	// if err := schema.Validate(); err != nil {
	// 	return fmt.Errorf("invalid schema: %w", err)
	// }

	return nil
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
