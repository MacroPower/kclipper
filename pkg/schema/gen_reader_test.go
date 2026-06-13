package schema_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/schema"
)

func TestReaderGenerator(t *testing.T) {
	t.Parallel()

	generator := schema.DefaultReaderGenerator

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
		"FileRefsEscapedPointers": {
			filePaths:    []string{"input/refs-escapes.schema.json"},
			expectedPath: "output/refs-escapes.schema.json",
		},
		"FileRefsInDefinitions": {
			filePaths:    []string{"input/refs-in-defs.schema.json"},
			expectedPath: "output/refs-in-defs.schema.json",
		},
		"DeepSchema": {
			filePaths:    []string{"input/deep.schema.json"},
			expectedPath: "output/deep.schema.json",
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

func TestReaderGeneratorCircularRefs(t *testing.T) {
	t.Parallel()

	generator := schema.DefaultReaderGenerator

	_, err := generator.FromPaths(filepath.Join(testDataDir, "input/cycle-a.schema.json"))
	require.ErrorContains(t, err, "circular reference")
}

func TestReaderGeneratorYAMLRefTarget(t *testing.T) {
	t.Parallel()

	generator := schema.DefaultReaderGenerator

	// A $ref target written as YAML resolves the same as one written as JSON.
	got, err := generator.FromPaths(filepath.Join(testDataDir, "input/refs-to-yaml.schema.json"))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"properties": {
			"foo": {"type": "string", "title": "from-yaml"}
		}
	}`, string(got))
}

func TestReaderGeneratorRefFailurePolicy(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input   string
		want    string
		wantErr bool
	}{
		// A fragment ref whose target is absent (a partial import) is dropped,
		// leaving the rest of the node intact.
		"fragment ref dropped": {
			input: `{"properties": {"foo": {"$ref": "#/definitions/missing", "title": "kept"}}}`,
			want:  `{"properties": {"foo": {"title": "kept"}}}`,
		},
		// An unresolved ref under an additionalProperties keyword fails open to
		// the permissive empty schema (which marshals to true) rather than
		// aborting the schema.
		"additionalProperties ref dropped": {
			input: `{"properties": {"x": {"additionalProperties": {"$ref": "missing.json#/a"}}}}`,
			want:  `{"properties": {"x": {"additionalProperties": true}}}`,
		},
		// A property literally named additionalProperties is a member, not the
		// keyword, so an unresolvable external ref there is fatal.
		"property named additionalProperties errors": {
			input:   `{"properties": {"additionalProperties": {"$ref": "missing.json#/a"}}}`,
			wantErr: true,
		},
		// An unresolvable external ref at a fixed position is fatal.
		"external ref errors": {
			input:   `{"properties": {"foo": {"$ref": "missing.json#/a"}}}`,
			wantErr: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := schema.DefaultReaderGenerator.FromData([]byte(tc.input), "")
			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestReaderGeneratorStripsUnknownMembers(t *testing.T) {
	t.Parallel()

	generator := schema.DefaultReaderGenerator

	// Unknown members (like wrapper keys in malformed vendored schemas) are
	// not JSON Schema keywords; they validate nothing and must not survive
	// into the output as pseudo-constraints.
	input := `{
		"properties": {
			"securityContext": {
				"io.k8s.api.core.v1.SecurityContext": {
					"type": "object",
					"properties": {"runAsUser": {"type": "integer"}}
				}
			},
			"foo": {"type": "string", "x-custom": "annotation"}
		}
	}`

	got, err := generator.FromData([]byte(input), "")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"properties": {
			"securityContext": true,
			"foo": {"type": "string"}
		}
	}`, string(got))
}

func TestReaderGeneratorFromDataYAML(t *testing.T) {
	t.Parallel()

	generator := schema.DefaultReaderGenerator

	jsonSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"properties": {"foo": {"type": "string"}}
	}`
	yamlSchema := "$schema: http://json-schema.org/draft-07/schema#\nproperties:\n  foo:\n    type: string\n"

	fromJSON, err := generator.FromData([]byte(jsonSchema), "")
	require.NoError(t, err)

	fromYAML, err := generator.FromData([]byte(yamlSchema), "")
	require.NoError(t, err)

	require.JSONEq(t, string(fromJSON), string(fromYAML))
}
