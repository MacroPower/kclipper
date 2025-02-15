package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/internal/cli"
)

var testDataDir string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testDataDir = filepath.Join(dir, "testdata")

	os.Setenv("KCLX_HELM_PLUGIN_DISABLED", "true")
	os.Setenv("KCLX_OS_PLUGIN_DISABLED", "true")
	os.Setenv("KCLX_HTTP_PLUGIN_DISABLED", "true")
}

func TestRun(t *testing.T) {
	t.Parallel()

	tc := cli.NewRootCmd("test", "", "")
	out := bytes.NewBufferString("")
	outFile := filepath.Join(testDataDir, "got/simple.json")
	err := os.MkdirAll(filepath.Dir(outFile), 0o755)
	require.NoError(t, err)

	tc.SetArgs([]string{"run", filepath.Join(testDataDir, "simple.k"), "--format=json", "--output", outFile})
	tc.SetOut(out)

	err = tc.Execute()
	require.NoError(t, err)
	require.Empty(t, out.String())

	outData, err := os.ReadFile(outFile)
	require.NoError(t, err)

	require.JSONEq(t, `{"a":1}`, string(outData))
}

func BenchmarkRun(b *testing.B) {
	for range b.N {
		tc := cli.NewRootCmd("test", "", "")

		// plugin.RegisterPlugin(helm.HelmPlugin)

		out := bytes.NewBufferString("")

		tc.SetArgs([]string{"run", filepath.Join(testDataDir, "simple.k"), "--output=/dev/null"})
		tc.SetOut(out)
		err := tc.Execute()
		require.NoError(b, err)
		require.Empty(b, out.String())
	}
}
