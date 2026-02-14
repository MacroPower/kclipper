package commands

import (
	"os"
	"strings"

	filepathplugin "github.com/macropower/kclipper/pkg/kclplugin/filepath"
	helmplugin "github.com/macropower/kclipper/pkg/kclplugin/helm"
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
