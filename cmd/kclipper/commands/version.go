package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	kclversion "kcl-lang.io/cli/pkg/version"

	"github.com/macropower/kclipper/pkg/version"
)

func GetVersionString() string {
	return fmt.Sprintf("%s+%s", version.Version, kclversion.GetVersionString())
}

// NewVersionCmd returns the version command.
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version of the kclipper CLI",
		Run: func(cc *cobra.Command, _ []string) {
			cc.Println(GetVersionString())
		},
	}
}
