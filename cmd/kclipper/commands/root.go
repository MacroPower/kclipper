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

	"github.com/macropower/kclipper/pkg/log"
)

var (
	ErrLogHandlerFailed = errors.New("log handler failed")

	heapProfile   *pprof.Profile
	allocsProfile *pprof.Profile
	blockProfile  *pprof.Profile
	mutexProfile  *pprof.Profile
)

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
	cmd.PersistentFlags().StringVar(args.heapProfile, "heapprofile", "", "Write a heap profile to this file")
	cmd.PersistentFlags().StringVar(args.memProfile, "memprofile", "", "Write a memory profile to this file")
	cmd.PersistentFlags().
		IntVar(args.memProfileRate, "memprofile_rate", 512*1024, "Memory profiling rate as a fraction")
	cmd.PersistentFlags().StringVar(args.blockProfile, "blockprofile", "", "Write a block profile to this file")
	cmd.PersistentFlags().IntVar(args.blockProfileRate, "blockprofile_rate", 1, "Block profiling rate as a fraction")
	cmd.PersistentFlags().StringVar(args.mutexProfile, "mutexprofile", "", "Write a mutex profile to this file")
	cmd.PersistentFlags().IntVar(args.mutexProfileRate, "mutexprofile_rate", 1, "Mutex profiling rate as a fraction")

	err := cmd.MarkPersistentFlagFilename("cpuprofile")
	if err != nil {
		panic(err)
	}

	err = cmd.MarkPersistentFlagFilename("heapprofile")
	if err != nil {
		panic(err)
	}

	err = cmd.MarkPersistentFlagFilename("memprofile")
	if err != nil {
		panic(err)
	}

	err = cmd.MarkPersistentFlagFilename("blockprofile")
	if err != nil {
		panic(err)
	}

	err = cmd.MarkPersistentFlagFilename("mutexprofile")
	if err != nil {
		panic(err)
	}

	cmd.PersistentPreRunE = func(cc *cobra.Command, _ []string) error {
		// Start CPU profiling if file is specified.
		if args.GetCPUProfile() != "" {
			f, err := os.Create(args.GetCPUProfile())
			if err != nil {
				return fmt.Errorf("failed to create CPU profile: %w", err)
			}

			err = pprof.StartCPUProfile(f)
			if err != nil {
				must(f.Close())

				return fmt.Errorf("failed to start CPU profile: %w", err)
			}
		}

		if args.GetHeapProfile() != "" || args.GetMemProfile() != "" {
			runtime.MemProfileRate = args.GetMemProfileRate()
		}

		if args.GetHeapProfile() != "" {
			heapProfile = pprof.Lookup("heap")
		}

		if args.GetMemProfile() != "" {
			allocsProfile = pprof.Lookup("allocs")
		}

		if args.GetBlockProfile() != "" {
			runtime.SetBlockProfileRate(args.GetBlockProfileRate())

			blockProfile = pprof.Lookup("block")
		}

		if args.GetMutexProfile() != "" {
			runtime.SetMutexProfileFraction(args.GetMutexProfileRate())

			mutexProfile = pprof.Lookup("mutex")
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
		if heapProfile != nil {
			f, err := os.Create(args.GetHeapProfile())
			if err != nil {
				return fmt.Errorf("failed to create heap profile: %w", err)
			}

			err = heapProfile.WriteTo(f, 0)
			if err != nil {
				return fmt.Errorf("failed to write heap profile: %w", err)
			}

			must(f.Close())
		}

		// Write memory profile if file is specified.
		if allocsProfile != nil {
			f, err := os.Create(args.GetMemProfile())
			if err != nil {
				return fmt.Errorf("failed to create memory profile: %w", err)
			}

			runtime.GC() //nolint:revive // Get up-to-date statistics for the profile.

			err = allocsProfile.WriteTo(f, 0)
			if err != nil {
				return fmt.Errorf("failed to write memory profile: %w", err)
			}

			must(f.Close())
		}

		// Write block profile if file is specified.
		if blockProfile != nil {
			f, err := os.Create(args.GetBlockProfile())
			if err != nil {
				return fmt.Errorf("failed to create block profile: %w", err)
			}

			err = blockProfile.WriteTo(f, 0)
			if err != nil {
				return fmt.Errorf("failed to write block profile: %w", err)
			}

			must(f.Close())
		}

		// Write mutex profile if file is specified.
		if mutexProfile != nil {
			f, err := os.Create(args.GetMutexProfile())
			if err != nil {
				return fmt.Errorf("failed to create mutex profile: %w", err)
			}

			err = mutexProfile.WriteTo(f, 0)
			if err != nil {
				return fmt.Errorf("failed to write mutex profile: %w", err)
			}

			must(f.Close())
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
	cmd.AddCommand(NewExportCmd(args))

	return cmd
}
