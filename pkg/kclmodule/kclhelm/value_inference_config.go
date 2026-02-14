package kclhelm

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	"github.com/macropower/kclipper/pkg/jsonschema"
)

// ValueInferenceConfig defines configuration for value inference via dadav/helm-schema.
type ValueInferenceConfig struct {
	// Consider yaml which is commented out.
	UncommentYAMLBlocks bool `json:"uncommentYAMLBlocks,omitempty"`
	// Parse and use helm-docs comments.
	HelmDocsCompatibilityMode bool `json:"helmDocsCompatibilityMode,omitempty"`
	// Keep the helm-docs prefix (--) in the schema.
	KeepHelmDocsPrefix bool `json:"keepHelmDocsPrefix,omitempty"`
	// Keep the whole leading comment (default: cut at empty line).
	KeepFullComment bool `json:"keepFullComment,omitempty"`
	// Remove the global key from the schema.
	RemoveGlobal bool `json:"removeGlobal,omitempty"`
	// Skip auto-generation of Title.
	SkipTitle bool `json:"skipTitle,omitempty"`
	// Skip auto-generation of Description.
	SkipDescription bool `json:"skipDescription,omitempty"`
	// Skip auto-generation of Required.
	SkipRequired bool `json:"skipRequired,omitempty"`
	// Skip auto-generation of Default.
	SkipDefault bool `json:"skipDefault,omitempty"`
	// Skip auto-generation of AdditionalProperties.
	SkipAdditionalProperties bool `json:"skipAdditionalProperties,omitempty"`
}

func (c *ValueInferenceConfig) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}

	js := r.Reflect(reflect.TypeFor[ValueInferenceConfig]())

	js.SetProperty("skipRequired", jsonschema.WithDefault(true))

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
		UncommentYAMLBlocks:       c.UncommentYAMLBlocks,
		HelmDocsCompatibilityMode: c.HelmDocsCompatibilityMode,
		KeepHelmDocsPrefix:        c.KeepHelmDocsPrefix,
		KeepFullComment:           c.KeepFullComment,
		RemoveGlobal:              c.RemoveGlobal,
		SkipTitle:                 c.SkipTitle,
		SkipDescription:           c.SkipDescription,
		SkipRequired:              c.SkipRequired,
		SkipDefault:               c.SkipDefault,
		SkipAdditionalProperties:  c.SkipAdditionalProperties,
	}
}
