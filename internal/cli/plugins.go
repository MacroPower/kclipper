package cli

import (
	"os"
	"strings"

	helmplugin "github.com/MacroPower/kclipper/pkg/plugin/helm"
)

func RegisterEnabledPlugins() {
	if !envTrue("KCLX_HELM_PLUGIN_DISABLED") {
		helmplugin.Register()
	}
}

func envTrue(key string) bool {
	return strings.ToLower(os.Getenv(key)) == "true"
}
