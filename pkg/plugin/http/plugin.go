package http

import (
	"fmt"
	"os"
	"strings"
	"time"

	"kcl-lang.io/lib/go/plugin"

	"github.com/MacroPower/kclx/pkg/http"
	"github.com/MacroPower/kclx/pkg/kclutil"
)

func init() {
	if strings.ToLower(os.Getenv("KCLX_HTTP_PLUGIN_DISABLED")) == "true" {
		return
	}

	plugin.RegisterPlugin(plugin.Plugin{
		Name: "http",
		MethodMap: map[string]plugin.MethodSpec{
			"get": {
				// http.get(url, timeout="30s")
				Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
					safeArgs := kclutil.SafeMethodArgs{Args: args}

					urlArg := args.StrArg(0)
					timeout := safeArgs.StrKwArg("timeout", "30s")
					timeoutDuration, err := time.ParseDuration(timeout)
					if err != nil {
						return nil, fmt.Errorf("failed to parse timeout %s: %w", timeout, err)
					}
					client := http.NewClient(timeoutDuration)
					body, status, err := client.Get(urlArg)
					if err != nil {
						return nil, fmt.Errorf("failed to get '%s': %w", urlArg, err)
					}

					return &plugin.MethodResult{V: map[string]any{
						"status": status,
						"body":   string(body),
					}}, nil
				},
			},
		},
	})
}
