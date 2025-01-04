package cli

import (
	"os"
	"strings"

	helmplugin "github.com/MacroPower/kclipper/pkg/plugin/helm"
	httpplugin "github.com/MacroPower/kclipper/pkg/plugin/http"
	osplugin "github.com/MacroPower/kclipper/pkg/plugin/os"
)

func RegisterEnabledPlugins() {
	if !envTrue("KCLX_HELM_PLUGIN_DISABLED") {
		helmplugin.Register()
	}
	if !envTrue("KCLX_HTTP_PLUGIN_DISABLED") {
		httpplugin.Register()
	}
	if !envTrue("KCLX_OS_PLUGIN_DISABLED") {
		osplugin.Register()
	}
}

func envTrue(key string) bool {
	return strings.ToLower(os.Getenv(key)) == "true"
}
