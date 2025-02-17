package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/internal/cli"
)

func TestChartCmd(t *testing.T) {
	t.Parallel()

	err := os.RemoveAll(filepath.Join(testDataDir, "got/chart_cmd"))
	require.NoError(t, err)

	tc := cli.NewRootCmd("test_chart", "", "")
	out := bytes.NewBufferString("")

	tc.SetArgs([]string{
		"chart", "init",
		"--path", filepath.Join(testDataDir, "got/chart_cmd"),
	})
	tc.SetOut(out)
	tc.SetErr(out)
	err = tc.Execute()
	require.NoError(t, err)
	require.Empty(t, out.String())
	out.Reset()

	// Replace `modules/helm` in kcl.mod with the correct path.
	modFile := filepath.Join(testDataDir, "got/chart_cmd/kcl.mod")
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
		"--path", filepath.Join(testDataDir, "got/chart_cmd"),
		"--chart=podinfo",
		"--target_revision=6.7.0",
		"--repo_url=https://stefanprodan.github.io/podinfo",
	})
	tc.SetOut(out)
	tc.SetErr(out)
	err = tc.Execute()
	require.NoError(t, err)
	require.Empty(t, out.String())
	out.Reset()

	tc.SetArgs([]string{
		"chart", "set",
		"--path", filepath.Join(testDataDir, "got/chart_cmd"),
		"--chart=podinfo",
		"-O", "targetRevision=6.7.1",
	})
	tc.SetOut(out)
	tc.SetErr(out)
	err = tc.Execute()
	require.NoError(t, err)
	require.Empty(t, out.String())
	out.Reset()

	tc.SetArgs([]string{
		"chart", "repo", "add",
		"--path", filepath.Join(testDataDir, "got/chart_cmd"),
		"--name=stefanprodan",
		"--url=https://stefanprodan.github.io/podinfo",
	})
	tc.SetOut(out)
	tc.SetErr(out)
	err = tc.Execute()
	require.NoError(t, err)
	require.Empty(t, out.String())
	out.Reset()

	tc.SetArgs([]string{
		"chart", "update",
		"--path", filepath.Join(testDataDir, "got/chart_cmd"),
	})
	tc.SetOut(out)
	tc.SetErr(out)
	err = tc.Execute()
	require.NoError(t, err)
	require.Empty(t, out.String())
	out.Reset()
}
