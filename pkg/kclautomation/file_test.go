package kclautomation_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/kclautomation"
	"github.com/macropower/kclipper/pkg/kclerrors"
)

func TestFile_OverrideFile(t *testing.T) {
	t.Parallel()

	testDir := filepath.Join(t.TempDir(), "file_test")
	err := os.MkdirAll(testDir, 0o750)
	require.NoError(t, err)

	tcs := map[string]struct {
		err         error
		filePath    string
		specs       []string
		importPaths []string
	}{
		"new file with simple spec": {
			filePath:    filepath.Join(testDir, "new_file.k"),
			specs:       []string{"foo=bar"},
			importPaths: []string{},
			err:         nil,
		},
		"existing file with spec": {
			filePath:    filepath.Join(testDir, "existing_file.k"),
			specs:       []string{"foo=bar"},
			importPaths: []string{},
			err:         nil,
		},
		"file with import paths": {
			filePath:    filepath.Join(testDir, "import_file.k"),
			specs:       []string{"foo=bar"},
			importPaths: []string{"import pkg"},
			err:         nil,
		},
		"invalid file path": {
			filePath:    filepath.Join(testDir, "invalid/path/file.k"),
			specs:       []string{"foo=bar"},
			importPaths: []string{},
			err:         kclerrors.ErrWriteFile,
		},
	}

	// Create the existing file for testing
	existingFilePath := filepath.Join(testDir, "existing_file.k")
	err = os.WriteFile(existingFilePath, []byte("# Existing content"), 0o600)
	require.NoError(t, err)

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ok, err := kclautomation.File.OverrideFile(tc.filePath, tc.specs, tc.importPaths)

			if tc.err != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.err)

				return
			}

			require.NoError(t, err)
			assert.True(t, ok)

			// Verify the file exists
			_, err = os.Stat(tc.filePath)
			require.NoError(t, err)

			// Read the file content to verify
			content, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)
			assert.NotEmpty(t, content)

			// If we specified import paths, check if they were included
			if len(tc.importPaths) > 0 {
				for _, importPath := range tc.importPaths {
					assert.Contains(t, string(content), importPath)
				}
			}

			// Check if specs were included - we only check for the key part of the spec
			// since the exact formatting (spaces, etc.) may vary
			for _, spec := range tc.specs {
				key := spec
				if before, _, ok := strings.Cut(spec, "="); ok {
					key = before
				}

				assert.Contains(t, string(content), key)
			}
		})
	}
}
