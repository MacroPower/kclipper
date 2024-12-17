package os_test

import (
	"testing"

	"kcl-lang.io/kcl-go/pkg/plugin"

	_ "github.com/MacroPower/kclx/pkg/os"
)

func TestPluginExecStdout(t *testing.T) {
	t.Parallel()

	resultJSON := plugin.Invoke("kcl_plugin.os.exec", []interface{}{"echo", []string{"hello"}}, nil)
	if resultJSON != `{"stderr":"","stdout":"hello\n"}` {
		t.Fatal(resultJSON)
	}
}

func TestPluginExecError(t *testing.T) {
	t.Parallel()

	resultJSON := plugin.Invoke("kcl_plugin.os.exec", []interface{}{"bash", []string{"-c", "exit 1"}}, nil)
	if resultJSON != `{"__kcl_PanicInfo__":"failed to execute bash: exit status 1"}` {
		t.Fatal(resultJSON)
	}
}
