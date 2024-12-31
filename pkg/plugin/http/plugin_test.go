package http_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	require.Empty(t, result.GetErrMessage())

	resultMap := map[string]any{}
	err = json.Unmarshal([]byte(result.GetJsonResult()), &resultMap)
	require.NoError(t, err)

	status, ok := resultMap["status"].(float64)
	require.True(t, ok, "unexpected status type: %T", resultMap["status"])
	require.InDeltaf(t, 200.0, status, 0.1, "unexpected status: %v", resultMap["status"])
}
