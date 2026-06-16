package main

import (
	"context"
	"fmt"

	"dagger/ci/internal/dagger"

	"golang.org/x/sync/errgroup"
)

// Lint runs the lint gate (golangci-lint, go mod tidy check, prettier) inside
// the devbox environment, mirroring `task lint`.
//
// +check
func (m *Ci) Lint(ctx context.Context) error {
	return m.runTask(ctx, "lint")
}

// Test runs the unit tests (CGO + the netgo build tag) inside the devbox
// environment, mirroring `task go:test`.
//
// +check
func (m *Ci) Test(ctx context.Context) error {
	return m.runTask(ctx, "go:test")
}

// TestCoverage runs all tests with coverage profiling inside the devbox
// environment (mirroring `task go:test:cover`) and returns the coverage profile
// file.
func (m *Ci) TestCoverage() *dagger.File {
	return m.env().
		WithExec([]string{"devbox", "run", "--", "task", "go:test:cover"}).
		File(".test/coverage.txt")
}

// LintReleaser validates the GoReleaser configuration. Delegates to the
// shared [Goreleaser] toolchain, which mounts the source over a minimal git
// repo (the kclipper remote URL is configured at construction) because the
// goreleaser config references a git remote for homebrew/nix repository
// resolution.
//
// +check
func (m *Ci) LintReleaser(ctx context.Context) error {
	return m.Goreleaser.Check(ctx)
}

// LintActions lints the GitHub Actions workflows for security issues by
// composing the zizmor toolchain directly. zizmor is not on the devbox PATH, so
// this gate does not run through devbox. It pins .github/zizmor.yaml as the
// config path rather than relying on zizmor's auto-discovery.
//
// +check
func (m *Ci) LintActions(ctx context.Context) error {
	return m.Zizmor.Lint(ctx)
}

// LintKCLModules validates that all KCL modules under modules/ can be
// packaged correctly. Uses a placeholder version and runs kcl mod pkg
// for each module without pushing to any registry.
//
// +check
func (m *Ci) LintKCLModules(ctx context.Context) error {
	patched, names, err := m.patchedModulesDir(ctx, "0.0.0-check")
	if err != nil {
		return err
	}

	// Pin the binary and the runtime container to the same platform so the
	// built kcl matches the container's architecture regardless of the engine's
	// native arch (an arm64 engine would otherwise exec an amd64 binary).
	bin, err := m.Binary(ctx, kclModuleToolPlatform)
	if err != nil {
		return err
	}

	ctr := runtimeBase(kclModuleToolPlatform).
		WithFile("/usr/local/bin/kcl", bin).
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

// Security scans source dependencies for known vulnerabilities by composing the
// security toolchain (Trivy) directly. The scanned source is the `ci`
// toolchain's source, whose root dagger.json customization already excludes the
// build and cache directories.
//
// +check
func (m *Ci) Security(ctx context.Context) error {
	return m.Scanner.ScanSource(ctx)
}

// SecuritySourceSarif scans source dependencies for known vulnerabilities and
// returns the results as a SARIF file for upload to GitHub Code Scanning. Unlike
// [Ci.Security], it does not gate on findings: SARIF capture must produce the
// file even when vulnerabilities are present, so they can be published to the
// Security tab. It scans the same source as the gate.
func (m *Ci) SecuritySourceSarif() *dagger.File {
	return m.Scanner.ScanSourceSarif()
}

// SecurityImageSarif builds the runtime image and scans it for known
// vulnerabilities, returning the results as a SARIF file for upload to GitHub
// Code Scanning. It composes the release image builder ([Ci.BuildImages]) so it
// scans exactly what a release publishes, then scans the native linux/amd64
// variant (Dagger evaluates only that variant lazily). Unlike the gating scans
// it does not fail on findings, and it surfaces OS-layer CVEs (the runtime base
// and apt packages) that the source scan, seeing only Go modules, cannot.
func (m *Ci) SecurityImageSarif(
	ctx context.Context,
	// Version label for OCI metadata on the scanned image.
	// +default="0.0.0-scan"
	version string,
) (*dagger.File, error) {
	variants, err := m.BuildImages(ctx, version, nil)
	if err != nil {
		return nil, err
	}

	return m.Scanner.ScanImageSarif(variants[0]), nil
}

// LintRenovate validates the Renovate configuration with
// renovate-config-validator, installed at a pinned version in a Node container
// so the check is self-contained and Renovate can bump its own validator
// version. It is the one check that composes neither devbox nor a shared
// toolchain.
//
// +check
func (m *Ci) LintRenovate(ctx context.Context) error {
	_, err := dag.Container().
		From(renovateImage).
		WithMountedCache("/root/.npm", dag.CacheVolume(kclipperCacheNamespace+":npm")).
		WithExec([]string{"npm", "install", "-g", "renovate@" + renovateVersion}).
		WithMountedFile("/src/"+renovateConfig, m.Source.File(renovateConfig)).
		WithWorkdir("/src").
		WithExec([]string{"renovate-config-validator", renovateConfig}).
		Sync(ctx)
	return err
}

// ReleaseDryRun validates the full release pipeline without publishing.
// Builds snapshot binaries via GoReleaser, verifies each binary's architecture
// matches its target platform, and constructs multi-arch container images,
// catching cross-compilation failures, missing tool binaries, and image build
// errors that would surface only during a real release.
//
// For a fast goreleaser config-only check, see [Ci.LintReleaser].
func (m *Ci) ReleaseDryRun(ctx context.Context) error {
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
