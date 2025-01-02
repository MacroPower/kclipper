// Copyright The KCL Authors. All rights reserved.
//go:build cgo
// +build cgo

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	kclcmd "kcl-lang.io/cli/cmd/kcl/commands"
	"kcl-lang.io/cli/pkg/plugin"

	"github.com/MacroPower/kclx/pkg/log"
	_ "github.com/MacroPower/kclx/pkg/plugin/helm"
	_ "github.com/MacroPower/kclx/pkg/plugin/http"
	_ "github.com/MacroPower/kclx/pkg/plugin/os"
)

func init() {
	log.SetLogFormat("text")
	log.SetLogLevel("warn")
}

const (
	cmdName   = "kcl"
	shortDesc = "The KCL Extended Command Line Interface (CLI)."
	longDesc  = `The KCL Extended Command Line Interface (CLI).

KCL is an open-source, constraint-based record and functional language that
enhances the writing of complex configurations, including those for cloud-native
scenarios. The KCL website: https://kcl-lang.io
`
)

func main() {
	cmd := &cobra.Command{
		Use:           cmdName,
		Short:         shortDesc,
		Long:          longDesc,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       GetVersionString(),
	}
	cmd.AddCommand(kclcmd.NewRunCmd())
	cmd.AddCommand(kclcmd.NewLintCmd())
	cmd.AddCommand(kclcmd.NewDocCmd())
	cmd.AddCommand(kclcmd.NewFmtCmd())
	cmd.AddCommand(kclcmd.NewTestCmd())
	cmd.AddCommand(kclcmd.NewVetCmd())
	cmd.AddCommand(kclcmd.NewCleanCmd())
	cmd.AddCommand(kclcmd.NewImportCmd())
	cmd.AddCommand(kclcmd.NewModCmd())
	cmd.AddCommand(kclcmd.NewRegistryCmd())
	cmd.AddCommand(kclcmd.NewServerCmd())
	cmd.AddCommand(NewVersionCmd())
	cmd.AddCommand(NewChartCmd())
	bootstrapCmdPlugin(cmd, plugin.NewDefaultPluginHandler([]string{cmdName}))

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, strings.TrimLeft(err.Error(), "\n"))
		os.Exit(1)
	}
}

// executeRunCmd the run command for the root command.
func executeRunCmd(args []string) {
	cmd := kclcmd.NewRunCmd()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func isHelpOrVersionFlag(flag string) bool {
	return flag == "-h" || flag == "--help" || flag == "-v" || flag == "--version"
}

func bootstrapCmdPlugin(cmd *cobra.Command, pluginHandler plugin.PluginHandler) {
	if pluginHandler == nil {
		return
	}
	if len(os.Args) <= 1 {
		return
	}
	cmdPathPieces := os.Args[1:]

	// only look for suitable extension executables if
	// the specified command does not already exist
	// flags cannot be placed before plugin name
	if strings.HasPrefix(cmdPathPieces[0], "-") && !isHelpOrVersionFlag(cmdPathPieces[0]) {
		executeRunCmd(cmdPathPieces)
		return
	}

	foundCmd, _, err := cmd.Find(cmdPathPieces)
	if err == nil {
		return
	}

	// Also check the commands that will be added by Cobra.
	// These commands are only added once rootCmd.Execute() is called, so we
	// need to check them explicitly here.
	var cmdName string // first "non-flag" arguments
	for _, arg := range cmdPathPieces {
		if !strings.HasPrefix(arg, "-") {
			cmdName = arg
			break
		}
	}

	builtinSubCmdExist := false
	for _, cmd := range foundCmd.Commands() {
		if cmd.Name() == cmdName {
			builtinSubCmdExist = true
			break
		}
	}
	switch cmdName {
	// Don't search for a plugin
	case "help", "completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
	default:
		if !builtinSubCmdExist {
			if err := plugin.HandlePluginCommand(pluginHandler, cmdPathPieces, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			executeRunCmd(cmdPathPieces)
		}
	}
}
