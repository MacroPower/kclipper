package helm_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	argocli "github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/stretchr/testify/require"
	"kcl-lang.io/kcl-go/pkg/spec/gpyrpc"
	"kcl-lang.io/lib/go/native"

	_ "github.com/MacroPower/kclx/pkg/plugin/helm"
)

var testDataDir string

func init() {
	argocli.SetLogLevel("warn")

	//nolint:dogsled
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testDataDir = filepath.Join(dir, "testdata")
}

func TestPluginHelmTemplate(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		kclFile     string
		resultsFile string
	}{
		"Simple": {
			kclFile:     "input/simple.k",
			resultsFile: "output/simple.json",
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			inputKCLFile := filepath.Join(testDataDir, tc.kclFile)
			wantResultsFile := filepath.Join(testDataDir, tc.resultsFile)

			inputKCL, err := os.ReadFile(inputKCLFile)
			require.NoError(t, err)

			want, err := os.ReadFile(wantResultsFile)
			require.NoError(t, err)

			client := native.NewNativeServiceClient()
			result, err := client.ExecProgram(&gpyrpc.ExecProgram_Args{
				KFilenameList: []string{"main.k"},
				KCodeList:     []string{string(inputKCL)},
				Args:          []*gpyrpc.Argument{},
			})
			require.NoError(t, err)
			require.Empty(t, result.GetErrMessage())

			got := result.GetJsonResult()

			require.JSONEq(t, string(want), got)
		})
	}
}

func BenchmarkPluginHelmTemplate(b *testing.B) {
	inputKCLFile := filepath.Join(testDataDir, "input/simple.k")
	inputKCL, err := os.ReadFile(inputKCLFile)
	require.NoError(b, err)

	client := native.NewNativeServiceClient()
	_, err = client.ExecProgram(&gpyrpc.ExecProgram_Args{
		KFilenameList: []string{"main.k"},
		KCodeList:     []string{string(inputKCL)},
		Args:          []*gpyrpc.Argument{},
	})
	require.NoError(b, err)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client := native.NewNativeServiceClient()
			result, err := client.ExecProgram(&gpyrpc.ExecProgram_Args{
				KFilenameList: []string{"main.k"},
				KCodeList:     []string{string(inputKCL)},
				Args:          []*gpyrpc.Argument{},
			})
			require.NoError(b, err)
			require.Empty(b, result.GetErrMessage())
		}
	})
}
