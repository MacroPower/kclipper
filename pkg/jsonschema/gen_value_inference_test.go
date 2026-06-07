package jsonschema_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/jsonschema"
)

func TestNewValueInferenceGenerator(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		config *jsonschema.ValueInferenceConfig
		err    error
	}{
		"default config": {
			config: &jsonschema.ValueInferenceConfig{},
		},
		"explicit annotators": {
			config: &jsonschema.ValueInferenceConfig{
				Annotators: []string{
					jsonschema.HelmSchemaAnnotator,
					jsonschema.HelmValuesSchemaAnnotator,
					jsonschema.BitnamiAnnotator,
					jsonschema.HelmDocsAnnotator,
				},
				Strict: true,
			},
		},
		"unknown annotator": {
			config: &jsonschema.ValueInferenceConfig{
				Annotators: []string{"helm-schema", "nonexistent"},
			},
			err: jsonschema.ErrUnknownAnnotator,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g, err := jsonschema.NewValueInferenceGenerator(tc.config)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, g)
		})
	}
}

func TestValueInferenceGeneratorFromData(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		config *jsonschema.ValueInferenceConfig
		data   string
		want   string
		err    string
	}{
		"basic": {
			config: &jsonschema.ValueInferenceConfig{},
			data:   "key: value\n",
			want: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {"key": {"type": "string"}},
				"additionalProperties": true
			}`,
		},
		"multiple documents": {
			config: &jsonschema.ValueInferenceConfig{},
			data:   "a: 1\n---\nb: x\n",
			want: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {"a": {"type": "integer"}, "b": {"type": "string"}},
				"additionalProperties": true
			}`,
		},
		"strict": {
			config: &jsonschema.ValueInferenceConfig{Strict: true},
			data:   "a: 1\nnested:\n  b: x\n",
			want: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"a": {"type": "integer"},
					"nested": {
						"type": "object",
						"properties": {"b": {"type": "string"}},
						"additionalProperties": false
					}
				},
				"additionalProperties": false
			}`,
		},
		"annotator priority first wins": {
			config: &jsonschema.ValueInferenceConfig{
				Annotators: []string{jsonschema.BitnamiAnnotator, jsonschema.HelmDocsAnnotator},
			},
			data: "## @param foo bitnami description\n# -- helm-docs description\nfoo: 1\n",
			want: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {"foo": {"type": "integer", "description": "bitnami description"}},
				"additionalProperties": true
			}`,
		},
		"annotator priority reversed": {
			config: &jsonschema.ValueInferenceConfig{
				Annotators: []string{jsonschema.HelmDocsAnnotator, jsonschema.BitnamiAnnotator},
			},
			data: "## @param foo bitnami description\n# -- helm-docs description\nfoo: 1\n",
			want: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {"foo": {"type": "integer", "description": "helm-docs description"}},
				"additionalProperties": true
			}`,
		},
		"existing schema reference": {
			config: &jsonschema.ValueInferenceConfig{},
			data:   "# yaml-language-server: $schema=values.schema.json\nkey: value\n",
			err:    "schema reference already exists",
		},
		"infer defaults": {
			config: &jsonschema.ValueInferenceConfig{InferDefaults: true},
			data:   "replicas: 3\nimage:\n  tag: latest\n",
			want: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"replicas": {"type": "integer", "default": 3},
					"image": {
						"type": "object",
						"properties": {"tag": {"type": "string", "default": "latest"}},
						"additionalProperties": true
					}
				},
				"additionalProperties": true
			}`,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g, err := jsonschema.NewValueInferenceGenerator(tc.config)
			require.NoError(t, err)

			got, err := g.FromData([]byte(tc.data))
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)

				return
			}

			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestValueInferenceGenerator(t *testing.T) {
	t.Parallel()

	generator := jsonschema.DefaultValueInferenceGenerator

	testCases := map[string]struct {
		expectedPath string
		filePaths    []string
	}{
		"SingleFile": {
			filePaths:    []string{"input/values.yaml"},
			expectedPath: "output/values_schema.json",
		},
		"MultipleFiles": {
			filePaths:    []string{"input/values.yaml", "input/values-prod.yaml"},
			expectedPath: "output/values_merged_schema.json",
		},
		// Cross-file type conflicts follow magicschema union semantics: a null
		// value in one file widens the type from another to a [type, null]
		// union, and incompatible types drop the type constraint entirely.
		"MultipleFilesTypeConflict": {
			filePaths:    []string{"input/values-conflict.yaml", "input/values-conflict-prod.yaml"},
			expectedPath: "output/values_conflict_schema.json",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var testFilePaths []string

			for _, filePath := range tc.filePaths {
				testFilePath := filepath.Join(testDataDir, filePath)
				testFilePaths = append(testFilePaths, testFilePath)

				// Ensure test file exists.
				_, err := os.Stat(testFilePath)
				require.NoError(t, err)
			}

			// Test FromPaths.
			schemaBytes, err := generator.FromPaths(testFilePaths...)
			require.NoError(t, err)
			require.NotEmpty(t, schemaBytes)

			// Verify the output schema.
			wantFilePath := filepath.Join(testDataDir, tc.expectedPath)
			expectedSchema, err := os.ReadFile(wantFilePath)
			require.NoError(t, err)
			require.JSONEq(t,
				string(expectedSchema), string(schemaBytes),
				"Input: %s\nWant: %s", strings.Join(testFilePaths, ", "), wantFilePath,
			)
		})
	}
}
