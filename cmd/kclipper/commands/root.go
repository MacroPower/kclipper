package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/spf13/cobra"

	kclcmd "kcl-lang.io/cli/cmd/kcl/commands"

	"github.com/MacroPower/kclipper/pkg/log"
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

	cmd.PersistentFlags().StringVar(args.cpuProfile, "cpuprofile", "", "Write a CPU profile to this file")
	if err := cmd.MarkPersistentFlagFilename("cpuprofile"); err != nil {
		panic(err)
	}

	cmd.PersistentFlags().StringVar(args.heapProfile, "heapprofile", "", "Write a heap profile to this file")
	if err := cmd.MarkPersistentFlagFilename("heapprofile"); err != nil {
		panic(err)
	}

	cmd.PersistentFlags().StringVar(args.memProfile, "memprofile", "", "Write a memory profile to this file")
	cmd.PersistentFlags().IntVar(args.memProfileRate, "memprofile_rate", 0, "Memory profiling rate as a fraction")
	if err := cmd.MarkPersistentFlagFilename("memprofile"); err != nil {
		panic(err)
	}

	cmd.PersistentFlags().StringVar(args.blockProfile, "blockprofile", "", "Write a block profile to this file")
	cmd.PersistentFlags().IntVar(args.blockProfileRate, "blockprofile_rate", 0, "Block profiling rate as a fraction")
	if err := cmd.MarkPersistentFlagFilename("blockprofile"); err != nil {
		panic(err)
	}

	cmd.PersistentFlags().StringVar(args.mutexProfile, "mutexprofile", "", "Write a mutex profile to this file")
	cmd.PersistentFlags().IntVar(args.mutexProfileRate, "mutexprofile_rate", 0, "Mutex profiling rate as a fraction")
	if err := cmd.MarkPersistentFlagFilename("mutexprofile"); err != nil {
		panic(err)
	}

	cmd.PersistentPreRunE = func(cc *cobra.Command, _ []string) error {
		if args.GetMemProfileRate() > 0 {
			runtime.MemProfileRate = args.GetMemProfileRate()
		}

		if args.GetBlockProfileRate() > 0 {
			runtime.SetBlockProfileRate(args.GetBlockProfileRate())
		}

		if args.GetMutexProfileRate() > 0 {
			runtime.SetMutexProfileFraction(args.GetMutexProfileRate())
		}

		// Start CPU profiling if file is specified.
		if args.GetCPUProfile() != "" {
			f, err := os.Create(args.GetCPUProfile())
			if err != nil {
				return fmt.Errorf("failed to create CPU profile: %w", err)
			}
			if err := pprof.StartCPUProfile(f); err != nil {
				must(f.Close())

				return fmt.Errorf("failed to start CPU profile: %w", err)
			}
		}

		h, err := log.CreateHandlerWithStrings(
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

		// Stop CPU profiling if it was started.
		if args.GetCPUProfile() != "" {
			pprof.StopCPUProfile()
		}

		// Write heap profile if file is specified.
		if args.GetHeapProfile() != "" {
			f, err := os.Create(args.GetHeapProfile())
			if err != nil {
				return fmt.Errorf("failed to create heap profile: %w", err)
			}
			defer must(f.Close())
			if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
				return fmt.Errorf("failed to write heap profile: %w", err)
			}
		}

		// Write memory profile if file is specified.
		if args.GetMemProfile() != "" {
			f, err := os.Create(args.GetMemProfile())
			if err != nil {
				return fmt.Errorf("failed to create memory profile: %w", err)
			}
			defer must(f.Close())
			runtime.GC() //nolint:revive // Get up-to-date statistics for the profile.
			if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
				return fmt.Errorf("failed to write memory profile: %w", err)
			}
		}

		// Write block profile if file is specified.
		if args.GetBlockProfile() != "" {
			f, err := os.Create(args.GetBlockProfile())
			if err != nil {
				return fmt.Errorf("failed to create block profile: %w", err)
			}
			defer must(f.Close())
			if err := pprof.Lookup("block").WriteTo(f, 0); err != nil {
				return fmt.Errorf("failed to write block profile: %w", err)
			}
		}

		// Write mutex profile if file is specified.
		if args.GetMutexProfile() != "" {
			f, err := os.Create(args.GetMutexProfile())
			if err != nil {
				return fmt.Errorf("failed to create mutex profile: %w", err)
			}
			defer must(f.Close())
			if err := pprof.Lookup("mutex").WriteTo(f, 0); err != nil {
				return fmt.Errorf("failed to write mutex profile: %w", err)
			}
		}

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
