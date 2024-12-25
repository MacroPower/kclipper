package http_test

import (
	"encoding/json"
	"testing"

	"kcl-lang.io/kcl-go/pkg/spec/gpyrpc"
	"kcl-lang.io/lib/go/native"

	_ "github.com/MacroPower/kclx/pkg/http"
)

func TestPluginHttp(t *testing.T) {
	t.Parallel()

	code := `
import kcl_plugin.http

_http = http.get("https://example.com")

_http
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

	resultMap := map[string]any{}
	if err := json.Unmarshal([]byte(result.GetJsonResult()), &resultMap); err != nil {
		t.Fatal(err)
	}

	status, ok := resultMap["status"].(float64)
	if !ok {
		t.Fatalf("unexpected status type: %T", resultMap["status"])
	}

	if status != 200 {
		t.Fatalf("unexpected status: %v", resultMap["status"])
	}
}
