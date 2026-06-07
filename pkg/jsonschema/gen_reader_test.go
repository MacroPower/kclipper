package jsonschema_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/jsonschema"
)

func TestReaderGenerator(t *testing.T) {
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

	generator := jsonschema.DefaultReaderGenerator

	_, err := generator.FromPaths(filepath.Join(testDataDir, "input/cycle-a.schema.json"))
	require.ErrorContains(t, err, "circular reference")
}

func TestReaderGeneratorStripsUnknownMembers(t *testing.T) {
	t.Parallel()

	generator := jsonschema.DefaultReaderGenerator

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

	generator := jsonschema.DefaultReaderGenerator

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
