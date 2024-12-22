package os_test

import (
	"testing"

	"kcl-lang.io/kcl-go/pkg/spec/gpyrpc"
	"kcl-lang.io/lib/go/native"

	_ "github.com/MacroPower/kclx/pkg/os"
)

func TestPluginExecStdout(t *testing.T) {
	t.Parallel()

	code := `
import kcl_plugin.os

_cmd = os.exec("echo", ["hello"])

{result = _cmd}
`

	client := native.NewNativeServiceClient()
	result, err := client.ExecProgram(&gpyrpc.ExecProgram_Args{
		KFilenameList: []string{"main.k"},
		KCodeList:     []string{code},
		Args:          []*gpyrpc.Argument{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.GetErrMessage() != "" {
		t.Fatal(result.GetErrMessage())
	}

	if result.GetJsonResult() != `{"result": {"stderr": "", "stdout": "hello\n"}}` {
		t.Fatal(result.GetJsonResult())
	}
}
