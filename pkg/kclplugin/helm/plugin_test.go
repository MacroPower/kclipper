package helm_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kcl-lang.io/kcl-go/pkg/native"
	"kcl-lang.io/kcl-go/pkg/spec/gpyrpc"

	helmplugin "github.com/macropower/kclipper/pkg/kclplugin/helm"
)

var testDataDir string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testDataDir = filepath.Join(dir, "testdata")
}

//nolint:paralleltest,tparallel // Due to t.Chdir.
func TestPluginHelmTemplate(t *testing.T) {
	helmplugin.Register()

	workDir := testDataDir
	t.Chdir(workDir)

	tcs := map[string]struct {
		kclFile     string
		resultsFile string
	}{
		"Remote": {
			kclFile:     "input/remote.k",
			resultsFile: "output/remote.json",
		},
		"Local": {
			kclFile:     "input/local.k",
			resultsFile: "output/local.json",
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			wantResultsFile := filepath.Join(testDataDir, tc.resultsFile)
			want, err := os.ReadFile(wantResultsFile)
			require.NoError(t, err)

			client := native.NewNativeServiceClient()
			result, err := client.ExecProgram(&gpyrpc.ExecProgramArgs{
				KFilenameList: []string{tc.kclFile},
				WorkDir:       workDir,
				Args:          []*gpyrpc.Argument{},
			})
			require.NoError(t, err)
			require.Empty(t, result.GetErrMessage(), result.GetLogMessage())

			got := result.GetJsonResult()
			assert.JSONEq(t, string(want), got)
		})
	}
}
