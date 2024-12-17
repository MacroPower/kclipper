// Copyright The KCL Authors. All rights reserved.
//go:build cgo
// +build cgo

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	cmd "kcl-lang.io/cli/cmd/kcl/commands"

	_ "github.com/MacroPower/kclx/pkg/os"
)

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
	cmdx := &cobra.Command{
		Use:           cmdName,
		Short:         shortDesc,
		Long:          longDesc,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       GetVersionString(),
	}
	cmdx.AddCommand(cmd.NewRunCmd())
	cmdx.AddCommand(cmd.NewLintCmd())
	cmdx.AddCommand(cmd.NewDocCmd())
	cmdx.AddCommand(cmd.NewFmtCmd())
	cmdx.AddCommand(cmd.NewTestCmd())
	cmdx.AddCommand(cmd.NewVetCmd())
	cmdx.AddCommand(cmd.NewCleanCmd())
	cmdx.AddCommand(cmd.NewImportCmd())
	cmdx.AddCommand(cmd.NewModCmd())
	cmdx.AddCommand(cmd.NewRegistryCmd())
	cmdx.AddCommand(cmd.NewServerCmd())
	cmdx.AddCommand(NewVersionCmd())

	if err := cmdx.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, strings.TrimLeft(err.Error(), "\n"))
		os.Exit(1)
	}
}
