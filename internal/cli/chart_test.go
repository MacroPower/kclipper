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
		"--timeout=60s",
		"--quiet",
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
		"--timeout=60s",
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
		"--timeout=60s",
		"--quiet",
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
		"--timeout=60s",
		"--quiet",
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

func TestChartArgPointers(t *testing.T) {
	t.Parallel()

	rootArgs := cli.NewRootArgs()
	args := cli.NewChartArgs(rootArgs)

	// Test default values
	assert.Equal(t, "", args.GetPath())
	assert.Equal(t, "", args.GetLogLevel())
	assert.Equal(t, "", args.GetLogFormat())
	assert.False(t, args.GetQuiet())
	assert.False(t, args.GetVendor())
}

func TestChartCmdRequiredFlagErrors(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		args []string
	}{
		"missing required chart flag": {
			args: []string{
				"chart", "add",
				"--repo_url=https://example.com/charts",
			},
		},
		"missing required repo_url flag": {
			args: []string{
				"chart", "add",
				"--chart=test",
			},
		},
		"missing chart name in set": {
			args: []string{
				"chart", "set",
				"--overrides=val=123",
			},
		},
		"missing overrides in set": {
			args: []string{
				"chart", "set",
				"--chart=test",
			},
		},
		"missing repo name in repo add": {
			args: []string{
				"chart", "repo", "add",
				"--url=https://example.com/charts",
			},
		},
		"missing repo url in repo add": {
			args: []string{
				"chart", "repo", "add",
				"--name=test",
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootCmd := cli.NewRootCmd("test_chart_errors", "", "")
			rootCmd.SetArgs(tc.args)

			err := rootCmd.Execute()
			require.Error(t, err)
			assert.ErrorContains(t, err, "required flag(s)")
		})
	}
}

func TestChartCmdInvalidArgErrors(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		args []string
	}{
		"invalid timeout value": {
			args: []string{
				"chart", "init",
				"--timeout=invalid",
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootCmd := cli.NewRootCmd("test_chart_errors", "", "")
			rootCmd.SetArgs(tc.args)

			err := rootCmd.Execute()
			require.Error(t, err)
			assert.ErrorContains(t, err, "invalid argument")
		})
	}
}
