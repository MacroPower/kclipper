package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MacroPower/kclx/internal/version"
)

// NewVersionCmd returns the version command.
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "versionx",
		Short: "Show version of the KCL Extensions plugin",
		Run: func(*cobra.Command, []string) {
			fmt.Println("Version:\t", version.Version)
			fmt.Println("Branch:\t\t", version.Branch)
			fmt.Println("BuildUser:\t", version.BuildUser)
			fmt.Println("BuildDate:\t", version.BuildDate)
			fmt.Println("Revision:\t", version.Revision)
			fmt.Println("GoVersion:\t", version.GoVersion)
			fmt.Println("GoOS:\t\t", version.GoOS)
			fmt.Println("GoArch:\t\t", version.GoArch)
		},
		SilenceUsage: true,
	}
}
