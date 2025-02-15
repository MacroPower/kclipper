package kclutil_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	kclgen "kcl-lang.io/kcl-go/pkg/tools/gen"

	"github.com/MacroPower/kclipper/pkg/kclutil"
)

var testDataDir string

func init() {
	//nolint:dogsled
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testDataDir = filepath.Join(dir, "testdata")
}

func TestKCLConversion(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		opts         *kclutil.GenKclOptions
		filePath     string
		expectedPath string
	}{
		"SimpleSchema": {
			filePath:     "input/schema.json",
			expectedPath: "output/schema.k",
			opts: &kclutil.GenKclOptions{
				Mode:          kclgen.ModeJsonSchema,
				CastingOption: kclgen.OriginalName,
			},
		},
		"SimpleSchemaWithoutDefaults": {
			filePath:     "input/schema.json",
			expectedPath: "output/no-defaults.k",
			opts: &kclutil.GenKclOptions{
				Mode:           kclgen.ModeJsonSchema,
				CastingOption:  kclgen.OriginalName,
				RemoveDefaults: true,
			},
		},
		"SchemaWithMultilineValues": {
			filePath:     "input/multiline.schema.json",
			expectedPath: "output/multiline.k",
			opts: &kclutil.GenKclOptions{
				Mode:          kclgen.ModeJsonSchema,
				CastingOption: kclgen.OriginalName,
			},
		},
		"SchemaWithMultilineValuesWithoutDefaults": {
			filePath:     "input/multiline.schema.json",
			expectedPath: "output/multiline-no-defaults.k",
			opts: &kclutil.GenKclOptions{
				Mode:           kclgen.ModeJsonSchema,
				CastingOption:  kclgen.OriginalName,
				RemoveDefaults: true,
			},
		},
		"DeepSchema": {
			filePath:     "input/deep.schema.json",
			expectedPath: "output/deep-schema.k",
			opts: &kclutil.GenKclOptions{
				Mode:          kclgen.ModeJsonSchema,
				CastingOption: kclgen.OriginalName,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			inputFilePath := filepath.Join(testDataDir, tc.filePath)
			schemaBytes, err := os.ReadFile(inputFilePath)
			require.NoError(t, err)
			require.NotEmpty(t, schemaBytes)

			out := &bytes.Buffer{}
			err = kclutil.Gen.GenKcl(out, "chart", schemaBytes, tc.opts)
			require.NoError(t, err)

			got := out.String()

			wantFilePath := filepath.Join(testDataDir, tc.expectedPath)
			// os.WriteFile(wantFilePath, []byte(got), 0o600)
			expectedSchema, err := os.ReadFile(wantFilePath)
			require.NoError(t, err)

			want := string(expectedSchema)

			require.Equal(t, want, got, "Input: %s\nWant: %s", inputFilePath, wantFilePath)
		})
	}
}
