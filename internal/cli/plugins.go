package cli

import (
	"os"
	"strings"

	filepathplugin "github.com/MacroPower/kclipper/pkg/plugin/filepath"
	helmplugin "github.com/MacroPower/kclipper/pkg/plugin/helm"
)

func RegisterEnabledPlugins() {
	if !envTrue("KCLIPPER_HELM_PLUGIN_DISABLED") {
		helmplugin.Register()
	}
	if !envTrue("KCLIPPER_FILEPATH_PLUGIN_DISABLED") {
		filepathplugin.Register()
	}
}

func envTrue(key string) bool {
	return strings.ToLower(os.Getenv(key)) == "true"
}
