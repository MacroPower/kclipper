package kclhelm

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"slices"

	"github.com/macropower/kclipper/pkg/jsonschema"
)

// ValueInferenceConfig defines configuration for value inference via magicschema.
type ValueInferenceConfig struct {
	// Annotation parsers to enable, in priority order.
	Annotators []string `json:"annotators,omitempty"`
	// Set additionalProperties to false on objects in the generated schema.
	Strict bool `json:"strict,omitempty"`
	// Record observed YAML values as schema defaults when no annotation
	// provides one.
	InferDefaults bool `json:"inferDefaults,omitempty"`

	// Deprecated: magicschema parses the YAML AST directly, so commented-out
	// blocks are never considered. This option is ignored.
	UncommentYAMLBlocks bool `json:"uncommentYAMLBlocks,omitempty"`
	// Deprecated: helm-docs comments are parsed by default via the helm-docs
	// annotator. Use annotators to control which parsers are enabled.
	HelmDocsCompatibilityMode bool `json:"helmDocsCompatibilityMode,omitempty"`
	// Deprecated: the helm-docs prefix is always stripped. This option is
	// ignored.
	KeepHelmDocsPrefix bool `json:"keepHelmDocsPrefix,omitempty"`
	// Deprecated: comment attribution is position-based and no longer
	// configurable. This option is ignored.
	KeepFullComment bool `json:"keepFullComment,omitempty"`
	// Deprecated: the global key is no longer special-cased. This option is
	// ignored.
	RemoveGlobal bool `json:"removeGlobal,omitempty"`
	// Deprecated: titles are never inferred from structure. This option is
	// ignored.
	SkipTitle bool `json:"skipTitle,omitempty"`
	// Deprecated: descriptions are only set from annotations. This option is
	// ignored.
	SkipDescription bool `json:"skipDescription,omitempty"`
	// Deprecated: required is never auto-generated. This option is ignored.
	SkipRequired bool `json:"skipRequired,omitempty"`
	// Deprecated: use inferDefaults instead (skipDefault is its inverse).
	SkipDefault bool `json:"skipDefault,omitempty"`
	// Deprecated: use strict instead. Magicschema keeps additionalProperties
	// open by default.
	SkipAdditionalProperties bool `json:"skipAdditionalProperties,omitempty"`
}

func (c *ValueInferenceConfig) GenerateKCL(w io.Writer) error {
	js, err := jsonschema.Reflect(reflect.TypeFor[ValueInferenceConfig](), jsonschema.WithGoComments())
	if err != nil {
		return fmt.Errorf("reflect schema: %w", err)
	}

	js.SetProperty("annotators", jsonschema.WithItemsEnum(jsonschema.AnnotatorEnum))
	js.SetProperty("inferDefaults", jsonschema.WithDefault(true))

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

// GetConfig adapts the KCL-facing config to the magicschema-facing
// [jsonschema.ValueInferenceConfig]. Legacy fields are mapped to their new
// equivalents where one exists; fields with no equivalent are accepted but
// ignored. Any legacy field that is set emits a deprecation warning.
func (c *ValueInferenceConfig) GetConfig() *jsonschema.ValueInferenceConfig {
	cfg := &jsonschema.ValueInferenceConfig{
		Annotators:    c.Annotators,
		Strict:        c.Strict,
		InferDefaults: c.InferDefaults,
	}

	// Mapped legacy fields: translate to the new behavior.
	if c.SkipDefault {
		slog.Warn("valueInference.skipDefault is deprecated; set inferDefaults=False instead")

		cfg.InferDefaults = false
	}

	if c.HelmDocsCompatibilityMode {
		slog.Warn(
			"valueInference.helmDocsCompatibilityMode is deprecated; helm-docs is enabled by default, use annotators to control which parsers run",
		)

		// Only meaningful when a custom annotator list omits helm-docs. An
		// empty list falls back to the defaults, which already include it.
		if len(cfg.Annotators) > 0 && !slices.Contains(cfg.Annotators, jsonschema.HelmDocsAnnotator) {
			cfg.Annotators = append(cfg.Annotators, jsonschema.HelmDocsAnnotator)
		}
	}

	// No-op legacy fields: accepted for backwards compatibility but ignored.
	if c.SkipAdditionalProperties {
		slog.Warn(
			"valueInference.skipAdditionalProperties is deprecated; use strict instead (additionalProperties stays open by default)",
		)
	}

	if c.SkipRequired {
		slog.Warn("valueInference.skipRequired is deprecated and ignored; required is never auto-generated")
	}

	if c.UncommentYAMLBlocks {
		slog.Warn(
			"valueInference.uncommentYAMLBlocks is deprecated and ignored; magicschema parses the YAML AST directly",
		)
	}

	if c.KeepFullComment {
		slog.Warn("valueInference.keepFullComment is deprecated and ignored; comment attribution is position-based")
	}

	if c.KeepHelmDocsPrefix {
		slog.Warn(
			"valueInference.keepHelmDocsPrefix is deprecated and ignored; the helm-docs prefix is always stripped",
		)
	}

	if c.RemoveGlobal {
		slog.Warn("valueInference.removeGlobal is deprecated and ignored; the global key is no longer special-cased")
	}

	if c.SkipTitle {
		slog.Warn("valueInference.skipTitle is deprecated and ignored; titles are never inferred from structure")
	}

	if c.SkipDescription {
		slog.Warn(
			"valueInference.skipDescription is deprecated and ignored; descriptions are only set from annotations",
		)
	}

	return cfg
}
