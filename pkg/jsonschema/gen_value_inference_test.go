package jsonschema_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

func TestValueInferenceGenerator(t *testing.T) {
	t.Parallel()

	generator := jsonschema.DefaultValueInferenceGenerator

	testCases := map[string]struct {
		filePaths    []string
		expectedPath string
	}{
		"SingleFile": {
			filePaths:    []string{"input/values.yaml"},
			expectedPath: "output/values_schema.json",
		},
		"MultipleFiles": {
			filePaths:    []string{"input/values.yaml", "input/values-prod.yaml"},
			expectedPath: "output/values_merged_schema.json",
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
			schemaBytes, err := generator.FromPaths(testFilePaths...)
			require.NoError(t, err)
			require.NotEmpty(t, schemaBytes)

			// Verify the output schema
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
