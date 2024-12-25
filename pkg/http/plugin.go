package http

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"kcl-lang.io/lib/go/plugin"

	pluginutil "github.com/MacroPower/kclx/pkg/util/plugin"
)

func init() {
	plugin.RegisterPlugin(plugin.Plugin{
		Name: "http",
		MethodMap: map[string]plugin.MethodSpec{
			"get": {
				// http.get(url)
				Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
					safeArgs := pluginutil.SafeMethodArgs{Args: args}

					urlArg := args.StrArg(0)
					urlParsed, err := url.Parse(urlArg)
					if err != nil {
						return nil, fmt.Errorf("failed to parse url %s: %w", urlArg, err)
					}
					timeout := safeArgs.StrKwArg("timeout", "30s")
					timeoutDuration, err := time.ParseDuration(timeout)
					if err != nil {
						return nil, fmt.Errorf("failed to parse timeout %s: %w", timeout, err)
					}

					client := &http.Client{Timeout: timeoutDuration}
					resp, err := client.Do(&http.Request{
						Method: http.MethodGet,
						URL:    urlParsed,
					})
					if err != nil {
						return nil, fmt.Errorf("failed to get %s: %w", urlArg, err)
					}
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						return nil, fmt.Errorf("failed to read body for %s: %w", urlArg, err)
					}
					if err := resp.Body.Close(); err != nil {
						return nil, fmt.Errorf("failed to close body for %s: %w", urlArg, err)
					}

					return &plugin.MethodResult{V: map[string]any{
						"status": resp.StatusCode,
						"body":   string(bodyBytes),
					}}, nil
				},
			},
		},
	})
}
