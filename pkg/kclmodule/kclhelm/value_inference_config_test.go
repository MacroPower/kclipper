package kclhelm_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/jsonschema"
	"github.com/macropower/kclipper/pkg/kclmodule/kclhelm"
)

func TestValueInferenceConfigGetConfig(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		in   kclhelm.ValueInferenceConfig
		want jsonschema.ValueInferenceConfig
	}{
		"new fields pass through": {
			in: kclhelm.ValueInferenceConfig{
				Annotators:    []string{jsonschema.HelmSchemaAnnotator},
				Strict:        true,
				InferDefaults: true,
			},
			want: jsonschema.ValueInferenceConfig{
				Annotators:    []string{jsonschema.HelmSchemaAnnotator},
				Strict:        true,
				InferDefaults: true,
			},
		},
		"skipDefault disables inferDefaults": {
			in: kclhelm.ValueInferenceConfig{
				InferDefaults: true,
				SkipDefault:   true,
			},
			want: jsonschema.ValueInferenceConfig{
				InferDefaults: false,
			},
		},
		"helmDocsCompatibilityMode appends to custom annotators": {
			in: kclhelm.ValueInferenceConfig{
				Annotators:                []string{jsonschema.HelmSchemaAnnotator},
				HelmDocsCompatibilityMode: true,
			},
			want: jsonschema.ValueInferenceConfig{
				Annotators: []string{jsonschema.HelmSchemaAnnotator, jsonschema.HelmDocsAnnotator},
			},
		},
		"helmDocsCompatibilityMode leaves empty annotators untouched": {
			in: kclhelm.ValueInferenceConfig{
				HelmDocsCompatibilityMode: true,
			},
			want: jsonschema.ValueInferenceConfig{
				Annotators: nil,
			},
		},
		"helmDocsCompatibilityMode noop when already present": {
			in: kclhelm.ValueInferenceConfig{
				Annotators:                []string{jsonschema.HelmDocsAnnotator},
				HelmDocsCompatibilityMode: true,
			},
			want: jsonschema.ValueInferenceConfig{
				Annotators: []string{jsonschema.HelmDocsAnnotator},
			},
		},
		"skipAdditionalProperties is a noop": {
			in: kclhelm.ValueInferenceConfig{
				SkipAdditionalProperties: true,
			},
			want: jsonschema.ValueInferenceConfig{
				Strict:     false,
				Annotators: nil,
			},
		},
		"keepFullComment is a noop": {
			in: kclhelm.ValueInferenceConfig{
				KeepFullComment: true,
			},
			want: jsonschema.ValueInferenceConfig{
				Strict:     false,
				Annotators: nil,
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.in.GetConfig()
			require.NotNil(t, got)
			assert.Equal(t, tc.want.Annotators, got.Annotators)
			assert.Equal(t, tc.want.Strict, got.Strict)
			assert.Equal(t, tc.want.InferDefaults, got.InferDefaults)
		})
	}
}

//nolint:paralleltest // Swaps the default slog logger.
func TestValueInferenceConfigGetConfigWarnsOnLegacyField(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(prev)
	})

	cfg := kclhelm.ValueInferenceConfig{KeepFullComment: true}
	_ = cfg.GetConfig()

	assert.Contains(t, buf.String(), "keepFullComment is deprecated")
	assert.Contains(t, strings.ToLower(buf.String()), "warn")
}
