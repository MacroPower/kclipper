package jsonschema_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

var testDataDir string

func init() {
	//nolint:dogsled
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testDataDir = filepath.Join(dir, "testdata")
}

func TestGetGenerator(t *testing.T) {
	t.Parallel()

	require.IsType(t, jsonschema.DefaultAutoGenerator, jsonschema.GetGenerator(jsonschema.AutoGeneratorType))
	require.IsType(t, jsonschema.DefaultNoGenerator, jsonschema.GetGenerator("UNKNOWN"))
}

func TestKCLConversion(t *testing.T) {
	t.Parallel()

	generator := jsonschema.DefaultReaderGenerator

	testCases := map[string]struct {
		filePaths    []string
		expectedPath string
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

				// Ensure test file exists
				_, err := os.Stat(testFilePath)
				require.NoError(t, err)
			}

			// Test FromPaths
			t.Logf("Test FromPaths: %s", strings.Join(testFilePaths, ", "))
			schemaBytes, err := generator.FromPaths(testFilePaths...)
			require.NoError(t, err)
			require.NotEmpty(t, schemaBytes)

			fixedSchemaBytes, err := jsonschema.ConvertToKCLCompatibleJSONSchema(schemaBytes)
			require.NoError(t, err)

			// Verify the output schema
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
