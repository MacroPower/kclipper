package commands

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"go.jacobcolvin.com/x/log"
	"go.jacobcolvin.com/x/profile"

	kclcmd "kcl-lang.io/cli/cmd/kcl/commands"
)

var ErrLogHandlerFailed = errors.New("log handler failed")

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

	profileCfg := profile.NewConfig()
	profileCfg.Flags = profile.Flags{
		CPUProfile:           "cpuprofile",
		HeapProfile:          "heapprofile",
		AllocsProfile:        "memprofile",
		GoroutineProfile:     "goroutineprofile",
		ThreadcreateProfile:  "threadcreateprofile",
		BlockProfile:         "blockprofile",
		MutexProfile:         "mutexprofile",
		MemProfileRate:       "memprofile_rate",
		BlockProfileRate:     "blockprofile_rate",
		MutexProfileFraction: "mutexprofile_rate",
	}

	profileCfg.RegisterFlags(cmd.PersistentFlags())

	err := profileCfg.RegisterCompletions(cmd)
	if err != nil {
		panic(err)
	}

	profiler := profileCfg.NewProfiler()

	cmd.PersistentPreRunE = func(cc *cobra.Command, _ []string) error {
		err := profiler.Start()
		if err != nil {
			return err
		}

		h, err := log.NewHandlerFromStrings(
			cc.ErrOrStderr(),
			args.GetLogLevel(),
			args.GetLogFormat(),
		)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrLogHandlerFailed, err)
		}

		slog.SetDefault(slog.New(h))

		slog.Debug("ready to go")

		return nil
	}

	cmd.PersistentPostRunE = func(_ *cobra.Command, _ []string) error {
		slog.Debug("shutting down")

		return profiler.Stop()
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
	cmd.AddCommand(NewExportCmd(args))

	return cmd
}
