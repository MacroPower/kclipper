package kclhelm

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	"github.com/macropower/kclipper/pkg/jsonschema"
)

// ValueInferenceConfig defines configuration for value inference via magicschema.
type ValueInferenceConfig struct {
	// Comma-separated list of annotators to enable (e.g. "dadav,norwoodj,bitnami,losisin").
	// When empty, all built-in annotators are enabled.
	Annotators string `json:"annotators,omitempty"`
	// When true, sets additionalProperties to false on generated schemas.
	Strict bool `json:"strict,omitempty"`
}

func (c *ValueInferenceConfig) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}

	js := r.Reflect(reflect.TypeFor[ValueInferenceConfig]())

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	_, err = b.WriteTo(w)
	if err != nil {
		return fmt.Errorf("failed to write to KCL schema: %w", err)
	}

	return nil
}

func (c *ValueInferenceConfig) GetConfig() *jsonschema.ValueInferenceConfig {
	return &jsonschema.ValueInferenceConfig{
		Annotators: c.Annotators,
		Strict:     c.Strict,
	}
}
