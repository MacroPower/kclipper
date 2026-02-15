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

// ErrLogHandler indicates an error occurred while creating a log handler.
var ErrLogHandler = errors.New("create log handler")

// NewRootCmd creates the root [*cobra.Command] for the kclipper CLI.
func NewRootCmd(name, shortDesc, longDesc string) *cobra.Command {
	logCfg := log.NewConfig()
	logCfg.Flags = log.Flags{
		Level:  "log_level",
		Format: "log_format",
	}

	cmd := &cobra.Command{
		Use:          name,
		Short:        shortDesc,
		Long:         longDesc,
		SilenceUsage: true,
		Version:      GetVersionString(),
	}

	logCfg.RegisterFlags(cmd.PersistentFlags())

	// Override default log level from "info" to "warn".
	logCfg.Level = "warn"
	cmd.PersistentFlags().Lookup("log_level").DefValue = "warn"

	err := logCfg.RegisterCompletions(cmd)
	if err != nil {
		panic(err)
	}

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

	err = profileCfg.RegisterCompletions(cmd)
	if err != nil {
		panic(err)
	}

	profiler := profileCfg.NewProfiler()

	cmd.PersistentPreRunE = func(cc *cobra.Command, _ []string) error {
		err := profiler.Start()
		if err != nil {
			return err
		}

		h, err := logCfg.NewHandler(cc.ErrOrStderr())
		if err != nil {
			return fmt.Errorf("%w: %w", ErrLogHandler, err)
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
	cmd.AddCommand(NewChartCmd(logCfg))
	cmd.AddCommand(NewExportCmd())

	return cmd
}
