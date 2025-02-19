package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/internal/cli"
)

func TestChartCmd(t *testing.T) {
	t.Parallel()

	basePath := filepath.Join(testDataDir, "got/chart_cmd")

	err := os.RemoveAll(basePath)
	require.NoError(t, err)

	tc := cli.NewRootCmd("test_chart", "", "")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	tc.SetArgs([]string{
		"chart", "init",
		"--path", basePath,
	})
	tc.SetOut(stdout)
	tc.SetErr(stderr)

	err = tc.Execute()
	require.NoError(t, err)
	assert.Empty(t, stderr.String(), "stderr should be empty")
	assert.Empty(t, stdout.String(), "stdout should be empty")

	stdout.Reset()
	stderr.Reset()

	// Replace `modules/helm` in kcl.mod with the correct path.
	modFile := filepath.Join(basePath, "kcl.mod")
	modData, err := os.ReadFile(modFile)
	require.NoError(t, err)
	modData = bytes.ReplaceAll(modData,
		[]byte(`helm = { path = "../modules/helm" }`),
		[]byte(`helm = { path = "../../../../../modules/helm" }`),
	)
	err = os.WriteFile(modFile, modData, 0o600)
	require.NoError(t, err)

	tc.SetArgs([]string{
		"chart", "add",
		"--path", basePath,
		"--chart=podinfo",
		"--target_revision=6.7.0",
		"--repo_url=https://stefanprodan.github.io/podinfo",
	})
	tc.SetOut(stdout)
	tc.SetErr(stderr)

	err = tc.Execute()
	require.NoError(t, err)
	assert.Empty(t, stderr.String(), "stderr should be empty")
	assert.Empty(t, stdout.String(), "stdout should be empty")

	stdout.Reset()
	stderr.Reset()

	tc.SetArgs([]string{
		"chart", "set",
		"--path", basePath,
		"--chart=podinfo",
		"-O", "targetRevision=6.7.1",
	})
	tc.SetOut(stdout)
	tc.SetErr(stderr)

	err = tc.Execute()
	require.NoError(t, err)
	assert.Empty(t, stderr.String(), "stderr should be empty")
	assert.Empty(t, stdout.String(), "stdout should be empty")

	stdout.Reset()
	stderr.Reset()

	tc.SetArgs([]string{
		"chart", "repo", "add",
		"--path", basePath,
		"--name=stefanprodan",
		"--url=https://stefanprodan.github.io/podinfo",
	})
	tc.SetOut(stdout)
	tc.SetErr(stderr)

	err = tc.Execute()
	require.NoError(t, err)
	assert.Empty(t, stderr.String(), "stderr should be empty")
	assert.Empty(t, stdout.String(), "stdout should be empty")

	stdout.Reset()
	stderr.Reset()

	tc.SetArgs([]string{
		"chart", "update",
		"--path", basePath,
	})
	tc.SetOut(stdout)
	tc.SetErr(stderr)

	err = tc.Execute()
	require.NoError(t, err)
	assert.Empty(t, stderr.String(), "stderr should be empty")
	assert.Empty(t, stdout.String(), "stdout should be empty")

	stdout.Reset()
	stderr.Reset()
}
