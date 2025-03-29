package kclutil

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"

	kclgen "kcl-lang.io/kcl-go/pkg/tools/gen"
)

// Export is a concurrency-safe KCL exporter.
var Export = &export{}

// Exporter handles exporting KCL schemas to other formats.
type export struct {
	mu sync.Mutex
}

// KCLSchemaToJSONSchema exports the specified schema from the given package
// path as a JSON schema. Other schemas in the package are included as
// definitions in the output, allowing them to be referenced.
func (e *export) KCLSchemaToJSONSchema(pkgPath, schemaName string) ([]byte, error) {
	swagger, err := e.safeExportSwaggerV2Spec(pkgPath)
	if err != nil {
		return nil, err
	}

	spec := kclgen.SwaggerV2ToOpenAPIV3Spec(swagger)

	// Get all available schema keys for better error messages.
	schemaKeys := make([]string, 0, len(spec.Components.Schemas))
	for k := range spec.Components.Schemas {
		schemaKeys = append(schemaKeys, k)
	}

	ref, ok := spec.Components.Schemas[schemaName]
	if !ok {
		return nil, fmt.Errorf("%w: available schemas: %v", ErrSchemaNotFound, schemaKeys)
	}

	requiredDefinitions, err := getRequiredDefinitions(ref, spec)
	if err != nil {
		return nil, fmt.Errorf("missing required definitions: %w: available schemas: %v", err, schemaKeys)
	}

	refYAMLAny, err := ref.Value.MarshalYAML()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrYAMLMarshal, err)
	}

	refYAML, ok := refYAMLAny.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: unexpected type %T for schema YAML", ErrInvalidFormat, refYAMLAny)
	}

	// Add definitions to the schema for complete reference.
	refYAML["definitions"] = requiredDefinitions
	// Add $schema to the root of the schema to indicate it's a JSON Schema.
	refYAML["$schema"] = "http://json-schema.org/draft-07/schema#"

	// Format as JSON with proper indentation.
	jsonData, err := json.MarshalIndent(refYAML, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJSONMarshal, err)
	}

	// Replace `"type": "bool"` with `"type": "boolean"` for compatibility with JSON Schema spec.
	jsonData = []byte(strings.ReplaceAll(string(jsonData), `"type": "bool"`, `"type": "boolean"`))

	return jsonData, nil
}

func getRequiredDefinitions(ref *openapi3.SchemaRef, spec *openapi3.T) (map[string]*openapi3.SchemaRef, error) {
	requiredDefinitions := map[string]*openapi3.SchemaRef{}

	for _, v := range ref.Value.Properties {
		subDefs, err := getRequiredDefinitions(v, spec)
		if err != nil {
			return nil, err
		}
		maps.Copy(requiredDefinitions, subDefs)

		if v.Ref != "" {
			id := getDefinitionID(v.Ref)
			def, ok := spec.Components.Schemas[id]
			if !ok {
				return nil, fmt.Errorf("schema %q: %w", id, ErrSchemaNotFound)
			}
			requiredDefinitions[id] = def
		}

		if v.Value.Items != nil && v.Value.Items.Ref != "" {
			id := getDefinitionID(v.Value.Items.Ref)
			def, ok := spec.Components.Schemas[id]
			if !ok {
				return nil, fmt.Errorf("schema %q: %w", id, ErrSchemaNotFound)
			}
			requiredDefinitions[id] = def
		}
	}

	for _, v := range requiredDefinitions {
		subDefs, err := getRequiredDefinitions(v, spec)
		if err != nil {
			return nil, err
		}
		maps.Copy(requiredDefinitions, subDefs)
	}

	return requiredDefinitions, nil
}

func getDefinitionID(path string) string {
	return strings.TrimPrefix(path, "#/definitions/")
}

func (e *export) safeExportSwaggerV2Spec(pkgPath string) (*kclgen.SwaggerV2Spec, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	spec, err := kclgen.ExportSwaggerV2Spec(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("export Swagger v2 spec: %w", err)
	}

	return spec, nil
}
