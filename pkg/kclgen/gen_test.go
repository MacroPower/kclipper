package kclgen_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/kclerrors"
	"github.com/MacroPower/kclipper/pkg/kclgen"
)

var testDataDir string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testDataDir = filepath.Join(dir, "testdata")
}

// errorWriter implements io.Writer and always returns an error
type errorWriter struct{}

func (w *errorWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("simulated write error")
}

func TestKCLConversion(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		writer       io.Writer
		err          error
		opts         *kclgen.GenKclOptions
		filePath     string
		expectedPath string
	}{
		"simple schema": {
			filePath:     "input/schema.json",
			expectedPath: "output/schema.k",
			opts: &kclgen.GenKclOptions{
				Mode:           kclgen.ModeJSONSchema,
				CastingOption:  kclgen.OriginalName,
				RemoveDefaults: false,
			},
			writer: &bytes.Buffer{},
			err:    nil,
		},
		"simple schema without defaults": {
			filePath:     "input/schema.json",
			expectedPath: "output/no-defaults.k",
			opts: &kclgen.GenKclOptions{
				Mode:           kclgen.ModeJSONSchema,
				CastingOption:  kclgen.OriginalName,
				RemoveDefaults: true,
			},
			writer: &bytes.Buffer{},
			err:    nil,
		},
		"schema with multiline values": {
			filePath:     "input/multiline.schema.json",
			expectedPath: "output/multiline.k",
			opts: &kclgen.GenKclOptions{
				Mode:           kclgen.ModeJSONSchema,
				CastingOption:  kclgen.OriginalName,
				RemoveDefaults: false,
			},
			writer: &bytes.Buffer{},
			err:    nil,
		},
		"schema with multiline values without defaults": {
			filePath:     "input/multiline.schema.json",
			expectedPath: "output/multiline-no-defaults.k",
			opts: &kclgen.GenKclOptions{
				Mode:           kclgen.ModeJSONSchema,
				CastingOption:  kclgen.OriginalName,
				RemoveDefaults: true,
			},
			writer: &bytes.Buffer{},
			err:    nil,
		},
		"deep schema": {
			filePath:     "input/deep.schema.json",
			expectedPath: "output/deep-schema.k",
			opts: &kclgen.GenKclOptions{
				Mode:           kclgen.ModeJSONSchema,
				CastingOption:  kclgen.OriginalName,
				RemoveDefaults: false,
			},
			writer: &bytes.Buffer{},
			err:    nil,
		},
		"writer error": {
			filePath: "input/schema.json",
			opts: &kclgen.GenKclOptions{
				Mode:          kclgen.ModeJSONSchema,
				CastingOption: kclgen.OriginalName,
			},
			writer: &errorWriter{},
			err:    kclerrors.ErrWrite,
		},
		"invalid schema": {
			filePath: "input/invalid.json",
			opts: &kclgen.GenKclOptions{
				Mode:          kclgen.ModeJSONSchema,
				CastingOption: kclgen.OriginalName,
			},
			writer: &bytes.Buffer{},
			err:    kclerrors.ErrGenerateKCL,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Set up the input data
			var schemaBytes []byte
			var err error

			if tc.filePath != "" {
				inputFilePath := filepath.Join(testDataDir, tc.filePath)
				schemaBytes, err = os.ReadFile(inputFilePath)
				require.NoError(t, err)
				require.NotEmpty(t, schemaBytes)
			}

			// Use the provided writer or create a new buffer if not provided
			out := tc.writer
			if out == nil {
				out = &bytes.Buffer{}
			}

			// Call the function being tested
			err = kclgen.Gen.GenKcl(out, "chart", schemaBytes, tc.opts)

			// Check error expectations
			if tc.err != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.err)

				return
			}

			require.NoError(t, err)

			// For successful conversions, compare with expected output
			if buffer, ok := out.(*bytes.Buffer); ok && tc.expectedPath != "" {
				got := buffer.String()
				wantFilePath := filepath.Join(testDataDir, tc.expectedPath)

				// os.WriteFile(wantFilePath, []byte(got), 0o600)

				expectedSchema, err := os.ReadFile(wantFilePath)
				require.NoError(t, err)
				want := string(expectedSchema)
				require.Equal(t, want, got, "Input: %s\nWant: %s", tc.filePath, tc.expectedPath)
			}
		})
	}
}
