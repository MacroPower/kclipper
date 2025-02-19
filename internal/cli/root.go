package cli

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	kclcmd "kcl-lang.io/cli/cmd/kcl/commands"

	"github.com/MacroPower/kclipper/pkg/log"
)

// Global lock for KCL command creation.
var mu sync.Mutex

func NewRootCmd(name, shortDesc, longDesc string) *cobra.Command {
	mu.Lock()
	defer mu.Unlock()

	cmd := &cobra.Command{
		Use:           name,
		Short:         shortDesc,
		Long:          longDesc,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       GetVersionString(),
	}

	cmd.PersistentFlags().String("log.level", "warn", "Set the log level")
	cmd.PersistentFlags().String("log.format", "text", "Set the log format")

	cmd.PersistentPreRunE = func(cc *cobra.Command, _ []string) error {
		flags := cc.Flags()

		var merr error

		logLevel, err := flags.GetString("log.level")
		if err != nil {
			merr = multierror.Append(merr, err)
		}

		logFormat, err := flags.GetString("log.format")
		if err != nil {
			merr = multierror.Append(merr, err)
		}

		if merr != nil {
			return fmt.Errorf("invalid argument: %w", merr)
		}

		h, err := log.CreateHandler(logLevel, logFormat)
		if err != nil {
			return fmt.Errorf("failed creating log handler: %w", err)
		}
		slog.SetDefault(slog.New(h))

		return nil
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

	return cmd
}
