// Copyright The KCL Authors. All rights reserved.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	kclcmd "kcl-lang.io/cli/cmd/kcl/commands"
	"kcl-lang.io/cli/pkg/plugin"

	"github.com/MacroPower/kclipper/internal/cli"
	"github.com/MacroPower/kclipper/pkg/log"
)

func init() {
	log.SetLogFormat("text")
	log.SetLogLevel("warn")
}

const (
	cmdName = "kcl"

	shortDesc = "The Kclipper Command Line Interface (CLI)."
	longDesc  = `The Kclipper (KCL + Helm) Command Line Interface (CLI).

KCL is an open-source, constraint-based record and functional language that
enhances the writing of complex configurations, including those for cloud-native
scenarios.

Kclipper combines KCL and Helm. It provides KCL plugins, packages, and
additional commands, which collectively allow you to manage Helm charts and
their schemas declaratively, and render Helm charts directly within KCL.

The KCL website: https://kcl-lang.io
`
)

func main() {
	cli.RegisterEnabledPlugins()

	cmd := cli.NewRootCmd(cmdName, shortDesc, longDesc)
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
