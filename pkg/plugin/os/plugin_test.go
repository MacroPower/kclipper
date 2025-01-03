package os_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"kcl-lang.io/kcl-go/pkg/spec/gpyrpc"
	"kcl-lang.io/lib/go/native"

	osplugin "github.com/MacroPower/kclipper/pkg/plugin/os"
)

func TestPluginExecStdout(t *testing.T) {
	t.Parallel()

	osplugin.Register()

	code := `
import kcl_plugin.os

_cmd = os.exec("echo", ["hello"])

{result = _cmd}
`
	want := `{"result": {"stderr": "", "stdout": "hello\n"}}`

	client := native.NewNativeServiceClient()
	result, err := client.ExecProgram(&gpyrpc.ExecProgram_Args{
		KFilenameList: []string{"main.k"},
		KCodeList:     []string{code},
		Args:          []*gpyrpc.Argument{},
	})
	require.NoError(t, err)
	require.Empty(t, result.GetErrMessage())

	got := result.GetJsonResult()

	require.JSONEq(t, want, got)
}
