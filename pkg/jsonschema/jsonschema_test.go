package jsonschema_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/jsonschema"
)

var testDataDir string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testDataDir = filepath.Join(dir, "testdata")
}

func TestConvertToKCLCompatibleJSONSchemaBoolSchemas(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input string
		want  string
	}{
		// The KCL gen tool errors on a boolean items schema; it must be
		// rewritten to an empty object schema.
		"boolean items": {
			input: `{"type": "object", "properties": {"a": {"type": "array", "items": true}}}`,
			want:  `{"type": "object", "properties": {"a": {"type": "array", "items": {}}}}`,
		},
		// The KCL gen tool mangles property names on boolean property
		// schemas; they must be rewritten to empty object schemas.
		"boolean property": {
			input: `{"type": "object", "properties": {"a": true}}`,
			want:  `{"type": "object", "properties": {"a": {}}}`,
		},
		// Boolean additionalProperties are supported by the KCL gen tool and
		// must be preserved.
		"boolean additionalProperties": {
			input: `{"type": "object", "additionalProperties": false, "properties": {"a": {"type": "string"}}}`,
			want:  `{"type": "object", "additionalProperties": false, "properties": {"a": {"type": "string"}}}`,
		},
		// The KCL gen tool renders an array without an items schema as any
		// instead of [any]; an empty items schema must be added.
		"array without items": {
			input: `{"type": "object", "properties": {"a": {"type": "array"}}}`,
			want:  `{"type": "object", "properties": {"a": {"type": "array", "items": {}}}}`,
		},
		// Type unions render better without an injected items schema.
		"array null union without items": {
			input: `{"type": "object", "properties": {"a": {"type": ["array", "null"]}}}`,
			want:  `{"type": "object", "properties": {"a": {"type": ["array", "null"]}}}`,
		},
		// Flattening compositors unions their enums; duplicate values that
		// land at non-adjacent positions must still be removed.
		"enum dedup across branches": {
			input: `{"anyOf": [{"enum": ["a", "b"]}, {"enum": ["a", "c"]}]}`,
			want:  `{"enum": ["a", "b", "c"]}`,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := jsonschema.ConvertToKCLCompatibleJSONSchema([]byte(tc.input))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestConvertToKCLSchemaBoolSchemas(t *testing.T) {
	t.Parallel()

	// Inferred schemas for null YAML values contain boolean property schemas;
	// the full conversion must absorb them without mangling attribute names.
	input := `{
		"type": "object",
		"properties": {
			"foo": true,
			"bar": {"type": "array", "items": true}
		}
	}`

	got, err := jsonschema.ConvertToKCLSchema([]byte(input), true)
	require.NoError(t, err)
	require.Contains(t, string(got), "foo?: any")
	require.Contains(t, string(got), "bar?: [any]")
}

func TestKCLConversion(t *testing.T) {
	t.Parallel()

	generator := jsonschema.DefaultReaderGenerator

	testCases := map[string]struct {
		expectedPath string
		filePaths    []string
	}{
		"SingleFile": {
			filePaths:    []string{"input/schema.json"},
			expectedPath: "output/schema.json",
		},
		"MultiFile": {
			filePaths:    []string{"input/nota.schema.json", "input/invalid.json", "input/schema.json"},
			expectedPath: "output/schema.json",
		},
		"FileRefs": {
			filePaths:    []string{"input/refs.schema.json"},
			expectedPath: "output/schema.json",
		},
		"DeepSchema": {
			filePaths:    []string{"input/deep.schema.json"},
			expectedPath: "output/deep-kcl.schema.json",
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
			t.Logf("Test FromPaths: %s", strings.Join(testFilePaths, ", "))

			schemaBytes, err := generator.FromPaths(testFilePaths...)
			require.NoError(t, err)
			require.NotEmpty(t, schemaBytes)

			fixedSchemaBytes, err := jsonschema.ConvertToKCLCompatibleJSONSchema(schemaBytes)
			require.NoError(t, err)

			// Verify the output schema.
			wantFilePath := filepath.Join(testDataDir, tc.expectedPath)
			expectedSchema, err := os.ReadFile(wantFilePath)
			require.NoError(t, err)
			require.JSONEq(t,
				string(expectedSchema), string(fixedSchemaBytes),
				"Input: %s\nWant: %s", strings.Join(testFilePaths, ", "), wantFilePath,
			)
		})
	}
}
