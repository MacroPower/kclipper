package commands_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/cmd/kclipper/commands"
)

func TestExportCmd(t *testing.T) {
	basePath := filepath.Join(testDataDir, "got/export_cmd")

	err := os.RemoveAll(basePath)
	require.NoError(t, err)

	tc := commands.NewRootCmd("test_export", "", "")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	tc.SetArgs([]string{
		"export", testDataDir,
		"--schema", "Test",
		"--output", filepath.Join(basePath, "schema.json"),
	})
	tc.SetOut(stdout)
	tc.SetErr(stderr)

	err = tc.Execute()
	require.NoError(t, err)
	assert.Empty(t, stderr.String(), "stderr should be empty")
	assert.Empty(t, stdout.String(), "stdout should be empty")
}
