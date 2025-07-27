// Copyright The KCL Authors. All rights reserved.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"kcl-lang.io/cli/pkg/plugin"

	kclcmd "kcl-lang.io/cli/cmd/kcl/commands"

	"github.com/macropower/kclipper/cmd/kclipper/commands"
)

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
	commands.RegisterEnabledPlugins()

	cmd := commands.NewRootCmd(cmdName, shortDesc, longDesc)

	ok, err := bootstrapCmdPlugin(cmd, plugin.NewDefaultPluginHandler([]string{cmdName}))
	if err != nil {
		fmt.Fprintln(os.Stderr, strings.TrimLeft(err.Error(), "\n"))
		os.Exit(1)
	}
	if ok {
		os.Exit(0)
	}

	err = cmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, strings.TrimLeft(err.Error(), "\n"))
		os.Exit(1)
	}

	os.Exit(0)
}

// executeRunCmd the run command for the root command.
func executeRunCmd(args []string) error {
	cmd := kclcmd.NewRunCmd()
	cmd.SetArgs(args)

	err := cmd.Execute()
	if err != nil {
		return fmt.Errorf("error executing run command: %w", err)
	}

	return nil
}

func isHelpOrVersionFlag(flag string) bool {
	return flag == "-h" || flag == "--help" || flag == "-v" || flag == "--version"
}

func bootstrapCmdPlugin(cmd *cobra.Command, pluginHandler plugin.PluginHandler) (bool, error) {
	if pluginHandler == nil {
		return false, nil
	}

	if len(os.Args) <= 1 {
		return false, nil
	}

	cmdPathPieces := os.Args[1:]

	// Only look for suitable extension executables if
	// the specified command does not already exist.
	// Flags cannot be placed before plugin name.
	if strings.HasPrefix(cmdPathPieces[0], "-") && !isHelpOrVersionFlag(cmdPathPieces[0]) {
		return true, executeRunCmd(cmdPathPieces)
	}

	foundCmd, _, err := cmd.Find(cmdPathPieces)
	if err == nil {
		return false, nil
	}

	// Also check the commands that will be added by Cobra.
	// These commands are only added once rootCmd.Execute() is called, so we
	// need to check them explicitly here.
	var cmdName string // First "non-flag" arguments.

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
	// Don't search for a plugin.
	case "help", "completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
	default:
		if !builtinSubCmdExist {
			err := plugin.HandlePluginCommand(pluginHandler, cmdPathPieces, false)
			if err != nil {
				return false, fmt.Errorf("error handling plugin command: %w", err)
			}

			return true, executeRunCmd(cmdPathPieces)
		}
	}

	return false, nil
}
