package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/internal/cli"
)

var testDataDir string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testDataDir = filepath.Join(dir, "testdata")
}

func TestRunCmd(t *testing.T) {
	t.Parallel()

	err := os.RemoveAll(filepath.Join(testDataDir, "got/run_cmd"))
	require.NoError(t, err)

	tc := cli.NewRootCmd("test_run", "", "")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	outFile := filepath.Join(testDataDir, "got/run_cmd/simple.json")
	err = os.MkdirAll(filepath.Dir(outFile), 0o750)
	require.NoError(t, err)

	tc.SetArgs([]string{
		"run", filepath.Join(testDataDir, "simple.k"),
		"--format=json",
		"--output", outFile,
	})
	tc.SetOut(stdout)
	tc.SetErr(stderr)

	err = tc.Execute()
	require.NoError(t, err)
	assert.Empty(t, stderr.String(), "stderr should be empty")
	assert.Empty(t, stdout.String(), "stdout should be empty")

	outData, err := os.ReadFile(outFile)
	require.NoError(t, err)

	require.JSONEq(t, `{"a":1}`, string(outData))
}

func BenchmarkRun(b *testing.B) {
	for range b.N {
		tc := cli.NewRootCmd("bench_run", "", "")
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		tc.SetArgs([]string{"run", filepath.Join(testDataDir, "simple.k"), "--output=/dev/null"})
		tc.SetOut(stdout)
		tc.SetErr(stderr)

		err := tc.Execute()
		require.NoError(b, err)
		assert.Empty(b, stderr.String(), "stderr should be empty")
		assert.Empty(b, stdout.String(), "stdout should be empty")
	}
}
