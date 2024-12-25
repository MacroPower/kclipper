package main

import (
	"github.com/spf13/cobra"
)

const (
	chartDesc = `This command manages kcl charts
`
	chartExample = `  kcl chart <command> [arguments]...
	# Initialize the current module
	kcl chart init

  # Add chart for the current module
  kcl chart add podinfo --repoURL https://stefanprodan.github.io/podinfo --targetRevision 6.7.0

  # Update chart schemas for the current module
  kcl chart update

  # Upgrade a chart and its values schema
  kcl chart upgrade podinfo --targetRevision 6.7.1`
)

// NewChartCmd returns the chart command.
func NewChartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "chart",
		Short:        "KCL chart management",
		Long:         chartDesc,
		Example:      chartExample,
		SilenceUsage: true,
	}
	cmd.AddCommand(NewChartAddCmd())
	cmd.AddCommand(NewChartUpdateCmd())
	cmd.AddCommand(NewChartUpgradeCmd())

	return cmd
}

func NewChartInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the current module",
		Run: func(*cobra.Command, []string) {
		},
		SilenceUsage: true,
	}
}

func NewChartAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new chart",
		Run: func(*cobra.Command, []string) {
		},
		SilenceUsage: true,
	}
}

func NewChartUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update charts",
		Run: func(*cobra.Command, []string) {
		},
		SilenceUsage: true,
	}
}

func NewChartUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade charts",
		Run: func(*cobra.Command, []string) {
		},
		SilenceUsage: true,
	}
}
