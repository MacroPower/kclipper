package main

import (
	"context"
	"fmt"

	"dagger/kclipper/internal/dagger"
	"golang.org/x/sync/errgroup"
)

// TestUnit runs the Go unit tests via the go toolchain. The shared go module
// does not mark TestUnit as a check, so this thin wrapper keeps unit tests in
// the kclipper check fan-out (dagger check, lefthook pre-commit, and CI).
//
// +check
func (m *Kclipper) TestUnit(ctx context.Context) error {
	return m.Go.TestUnit(ctx)
}

// LintReleaser validates the GoReleaser configuration. Delegates to the
// shared [Goreleaser] toolchain, which mounts the source over a minimal git
// repo (the kclipper remote URL is configured at construction) because the
// goreleaser config references a git remote for homebrew/nix repository
// resolution.
//
// +check
func (m *Kclipper) LintReleaser(ctx context.Context) error {
	return m.Goreleaser.Check(ctx)
}

// ReleaseDryRun validates the full release pipeline without publishing.
// Builds snapshot binaries via GoReleaser, verifies each binary's architecture
// matches its target platform, and constructs multi-arch container images,
// catching cross-compilation failures, missing tool binaries, and image build
// errors that would surface only during a real release.
//
// For a fast goreleaser config-only check, see [Kclipper.LintReleaser].
func (m *Kclipper) ReleaseDryRun(ctx context.Context) error {
	// platformBinaries maps each target platform to its GoReleaser dist path.
	type platformBinary struct {
		platform dagger.Platform
		distDir  string
	}
	targets := []platformBinary{
		{platform: "linux/amd64", distDir: "kclipper_linux_amd64_v1"},
		{platform: "linux/arm64", distDir: "kclipper_linux_arm64_v8.0"},
	}

	// Snapshot build — exercises goreleaser cross-compilation for all
	// platforms, releaserBase tool setup (cosign, syft, zig), and
	// archive/checksum generation.
	dist, err := m.Build(ctx)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)

	// Platform verification — asserts each binary is for the intended
	// architecture, catching cross-compilation mismatches early.
	for _, t := range targets {
		g.Go(func() error {
			bin := dist.File(t.distDir + "/kcl")
			if err := m.Goreleaser.VerifyBinaryPlatform(ctx, bin, t.platform); err != nil {
				return fmt.Errorf("platform verification for %s: %w", t.platform, err)
			}
			return nil
		})
	}

	// Container image build — exercises runtime base image construction,
	// binary packaging, and OCI metadata for all platforms.
	g.Go(func() error {
		containers, err := m.BuildImages(ctx, "dry-run", dist)
		if err != nil {
			return err
		}
		for _, ctr := range containers {
			if _, err := ctr.Sync(ctx); err != nil {
				return err
			}
		}
		return nil
	})

	return g.Wait()
}

// LintPrettier checks YAML, JSON, and Markdown formatting.
//
// +check
func (m *Kclipper) LintPrettier(
	ctx context.Context,
	// Prettier config file path relative to source root.
	// +optional
	configPath string,
	// File patterns to check.
	// +optional
	patterns []string,
) error {
	if configPath == "" {
		configPath = "./.prettierrc.yaml"
	}
	if len(patterns) == 0 {
		patterns = defaultPrettierPatterns()
	}
	args := append([]string{"prettier", "--config", configPath, "--check"}, patterns...)
	_, err := m.prettierBase().
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec(args).
		Sync(ctx)
	return err
}

// LintActions lints GitHub Actions workflows by delegating to the shared
// [Zizmor] toolchain, which runs zizmor against .github/workflows using the
// project's .github/zizmor.yaml configuration.
//
// +check
func (m *Kclipper) LintActions(ctx context.Context) error {
	return m.Zizmor.Lint(ctx)
}

// LintKCLModules validates that all KCL modules under modules/ can be
// packaged correctly. Uses a placeholder version and runs kcl mod pkg
// for each module without pushing to any registry.
//
// +check
func (m *Kclipper) LintKCLModules(ctx context.Context) error {
	patched, names, err := m.patchedModulesDir(ctx, "0.0.0-check")
	if err != nil {
		return err
	}

	ctr := runtimeBase("").
		WithFile("/usr/local/bin/kcl", m.Binary("")).
		WithMountedDirectory("/modules", patched).
		WithWorkdir("/modules")

	for _, name := range names {
		ctr = ctr.
			WithWorkdir("/modules/" + name).
			WithExec([]string{
				"kcl", "mod", "pkg", "--target", "/tmp/" + name + ".tar",
			})
	}

	_, err = ctr.Sync(ctx)
	return err
}

// LintDeadcode reports unreachable functions in the codebase by delegating to
// the shared [Go] toolchain's deadcode analyzer. This is an advisory lint that
// is not included in standard checks; invoke via dagger call kclipper lint-deadcode.
func (m *Kclipper) LintDeadcode(ctx context.Context) error {
	return m.Go.LintDeadcode(ctx)
}
