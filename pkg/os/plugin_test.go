package os_test

import (
	"testing"

	"kcl-lang.io/kcl-go/pkg/plugin"

	_ "github.com/MacroPower/kclx/pkg/os"
)

func TestPluginAdd(t *testing.T) {
	t.Parallel()

	resultJSON := plugin.Invoke("kcl_plugin.os.exec", []interface{}{"echo", []string{"hello"}}, nil)
	if resultJSON != `{"stderr":"","stdout":"hello\n"}` {
		t.Fatal(resultJSON)
	}
}
