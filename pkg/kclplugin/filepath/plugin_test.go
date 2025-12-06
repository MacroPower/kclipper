package filepathplugin_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kcl-lang.io/kcl-go/pkg/native"
	"kcl-lang.io/kcl-go/pkg/spec/gpyrpc"

	filepathplugin "github.com/macropower/kclipper/pkg/kclplugin/filepath"
)

func TestPluginFilepath(t *testing.T) {
	t.Parallel()

	filepathplugin.Register()

	tcs := map[string]struct {
		kclCode string
		want    string
	}{
		"base": {
			kclCode: `filepath.base("/path/to/file")`,
			want:    `"file"`,
		},
		"clean": {
			kclCode: `filepath.clean("/path/to/../file")`,
			want:    `"/path/file"`,
		},
		"dir": {
			kclCode: `filepath.dir("/path/to/file")`,
			want:    `"/path/to"`,
		},
		"ext": {
			kclCode: `filepath.ext("/path/to/file.txt")`,
			want:    `".txt"`,
		},
		"join": {
			kclCode: `filepath.join(["/path/to", "file"])`,
			want:    `"/path/to/file"`,
		},
		"split": {
			kclCode: `filepath.split("/path/to/file")`,
			want:    `["/path/to/", "file"]`,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := native.NewNativeServiceClient()
			result, err := client.ExecProgram(&gpyrpc.ExecProgramArgs{
				KFilenameList: []string{"main.k"},
				KCodeList: []string{
					"import kcl_plugin.filepath\n" +
						"result = " + tc.kclCode,
				},
				Args: []*gpyrpc.Argument{},
			})
			require.NoError(t, err)
			require.Empty(t, result.GetErrMessage(), result.GetLogMessage())

			want := fmt.Sprintf(`{"result": %s}`, tc.want)

			got := result.GetJsonResult()
			assert.JSONEq(t, want, got)
		})
	}
}
