package commands

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	kclcmd "kcl-lang.io/cli/cmd/kcl/commands"

	"github.com/MacroPower/kclipper/pkg/log"
)

var ErrLogHandlerFailed = errors.New("log handler failed")

type RootArgs struct {
	logLevel  *string
	logFormat *string
}

func NewRootArgs() *RootArgs {
	return &RootArgs{
		logLevel:  new(string),
		logFormat: new(string),
	}
}

func (a *RootArgs) GetLogLevel() string {
	return *a.logLevel
}

func (a *RootArgs) GetLogFormat() string {
	return *a.logFormat
}

func NewRootCmd(name, shortDesc, longDesc string) *cobra.Command {
	args := NewRootArgs()

	cmd := &cobra.Command{
		Use:           name,
		Short:         shortDesc,
		Long:          longDesc,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       GetVersionString(),
	}

	cmd.PersistentFlags().StringVar(args.logLevel, "log_level", "warn", "Set the log level (debug, info, warn, error)")
	cmd.PersistentFlags().StringVar(args.logFormat, "log_format", "text", "Set the log format (text, logfmt, json)")

	cmd.PersistentPreRunE = func(cc *cobra.Command, _ []string) error {
		h, err := log.CreateHandlerWithStrings(
			cc.OutOrStderr(),
			args.GetLogLevel(),
			args.GetLogFormat(),
		)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrLogHandlerFailed, err)
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
	cmd.AddCommand(NewChartCmd(args))

	return cmd
}
