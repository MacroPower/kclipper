package jsonschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.jacobcolvin.com/x/magicschema"
	"go.jacobcolvin.com/x/magicschema/helm"
)

var (
	// DefaultValueInferenceGenerator is an opinionated [ValueInferenceGenerator].
	DefaultValueInferenceGenerator = NewValueInferenceGenerator(&ValueInferenceConfig{})

	_ FileGenerator = DefaultValueInferenceGenerator
)

// ValueInferenceConfig configures schema generation via magicschema.
type ValueInferenceConfig struct {
	// Comma-separated list of annotators to enable (e.g. "dadav,norwoodj,bitnami,losisin").
	// When empty, all built-in annotators are enabled.
	Annotators string `json:"annotators,omitempty"`
	// When true, sets additionalProperties to false on generated schemas.
	Strict bool `json:"strict,omitempty"`
}

// ValueInferenceGenerator generates JSON Schema from Helm values files
// using magicschema.
//
// Create instances with [NewValueInferenceGenerator].
type ValueInferenceGenerator struct {
	gen *magicschema.Generator
}

// NewValueInferenceGenerator creates a new [ValueInferenceGenerator] using the
// given [ValueInferenceConfig].
func NewValueInferenceGenerator(c *ValueInferenceConfig) *ValueInferenceGenerator {
	opts, err := buildOptions(c)
	if err != nil {
		// Config validation errors should not happen with valid annotator names.
		// Fall back to a generator with no annotators if something goes wrong.
		return &ValueInferenceGenerator{gen: magicschema.NewGenerator()}
	}

	return &ValueInferenceGenerator{
		gen: magicschema.NewGenerator(opts...),
	}
}

func buildOptions(c *ValueInferenceConfig) ([]magicschema.Option, error) {
	var opts []magicschema.Option

	if c.Annotators != "" {
		registry := helm.DefaultRegistry()

		var annotators []magicschema.Annotator

		for name := range strings.SplitSeq(c.Annotators, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}

			a, ok := registry[name]
			if !ok {
				return nil, fmt.Errorf("%w: unknown annotator %q", magicschema.ErrInvalidOption, name)
			}

			annotators = append(annotators, a)
		}

		opts = append(opts, magicschema.WithAnnotators(annotators...))
	}

	if c.Strict {
		opts = append(opts, magicschema.WithStrict(true))
	}

	return opts, nil
}

// FromPaths generates a JSON Schema from one or more file paths pointing to
// Helm values files. If multiple file paths are provided, the schemas are
// merged into a single schema using union semantics.
func (g *ValueInferenceGenerator) FromPaths(paths ...string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, errors.New("no file paths provided")
	}

	inputs := make([][]byte, 0, len(paths))

	for _, path := range paths {
		//nolint:gosec // G304 not relevant for client-side generation.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read values file %q: %w", path, err)
		}

		inputs = append(inputs, data)
	}

	schema, err := g.gen.Generate(inputs...)
	if err != nil {
		return nil, fmt.Errorf("generate schema: %w", err)
	}

	out, err := marshalSchema(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	return out, nil
}

// FromData generates a JSON Schema from raw YAML data.
func (g *ValueInferenceGenerator) FromData(data []byte) ([]byte, error) {
	schema, err := g.gen.Generate(data)
	if err != nil {
		return nil, fmt.Errorf("generate schema: %w", err)
	}

	out, err := marshalSchema(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	return out, nil
}

// marshalSchema marshals a JSON Schema and normalizes boolean sub-schemas
// (true → {}, false → {"not":{}}) for compatibility with downstream consumers
// that cannot parse boolean schema values in object positions.
func marshalSchema(schema any) ([]byte, error) {
	return marshalSchemaWith(schema, json.Marshal)
}

// marshalSchemaIndent is like [marshalSchema] but produces indented JSON output.
func marshalSchemaIndent(schema any) ([]byte, error) {
	return marshalSchemaWith(schema, func(v any) ([]byte, error) {
		return json.MarshalIndent(v, "", "  ")
	})
}

func marshalSchemaWith(schema any, marshal func(any) ([]byte, error)) ([]byte, error) {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	var v any

	err = json.Unmarshal(data, &v)
	if err != nil {
		return nil, fmt.Errorf("unmarshal schema for normalization: %w", err)
	}

	normalizeBooleanSchemas(v)

	out, err := marshal(v)
	if err != nil {
		return nil, fmt.Errorf("re-marshal normalized schema: %w", err)
	}

	return out, nil
}

// normalizeBooleanSchemas recursively walks a parsed JSON value and replaces
// boolean sub-schemas with their object equivalents. In JSON Schema Draft 7+,
// true is equivalent to {} (validates everything) and false is equivalent to
// {"not":{}} (validates nothing). Some consumers (e.g. dadav/helm-schema)
// cannot parse boolean values in *Schema positions, so we expand them.
func normalizeBooleanSchemas(v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}

	// Fields that hold a single sub-schema (*Schema in Go).
	// Note: additionalProperties is excluded because true/false are
	// natural and expected boolean schemas for that field.
	for _, key := range []string{
		"items", "additionalItems", "contains",
		"not", "if", "then", "else",
		"propertyNames", "contentSchema",
		"unevaluatedItems", "unevaluatedProperties",
	} {
		if val, exists := m[key]; exists {
			m[key] = expandBooleanSchema(val)
		}
	}

	// Fields that hold a map of sub-schemas (map[string]*Schema).
	for _, key := range []string{
		"properties", "patternProperties", "definitions", "$defs", "dependentSchemas",
	} {
		if sub, ok := m[key].(map[string]any); ok {
			for k, val := range sub {
				sub[k] = expandBooleanSchema(val)
			}
		}
	}

	// Fields that hold an array of sub-schemas ([]*Schema).
	for _, key := range []string{
		"allOf", "anyOf", "oneOf", "prefixItems",
	} {
		if arr, ok := m[key].([]any); ok {
			for i, val := range arr {
				arr[i] = expandBooleanSchema(val)
			}
		}
	}

	// Recurse into all map values that are objects.
	for _, val := range m {
		normalizeBooleanSchemas(val)
	}
}

// expandBooleanSchema converts a boolean JSON Schema value to its object
// equivalent. Non-boolean values are returned as-is.
func expandBooleanSchema(v any) any {
	b, ok := v.(bool)
	if !ok {
		return v
	}

	if b {
		return map[string]any{}
	}

	return map[string]any{"not": map[string]any{}}
}
