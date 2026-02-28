// CI/CD functions specific to the kclipper project. Provides building,
// releasing, benchmarking, and development container support. Generic CI
// functions (testing, linting, formatting) are provided by the [Go] toolchain
// module; this module adds kclipper-specific logic that depends on the project
// clone URL, cross-compilation tooling, and runtime image configuration.

package main

import (
	"context"

	"dagger/kclipper/internal/dagger"
)

const (
	zigVersion    = "0.15.2"  // renovate: datasource=github-releases depName=ziglang/zig
	kclLSPVersion = "v0.11.2" // renovate: datasource=github-releases depName=kcl-lang/kcl

	defaultRegistry = "ghcr.io/macropower/kclipper"

	kclipperCloneURL = "https://github.com/macropower/kclipper.git"

	// macosSDKFlags are the common compiler flags for macOS cross-compilation
	// via Zig, pointing to the vendored macOS SDK headers.
	macosSDKFlags = "-F/sdk/MacOSX.sdk/System/Library/Frameworks " +
		"-I/sdk/MacOSX.sdk/usr/include " +
		"-L/sdk/MacOSX.sdk/usr/lib " +
		"-Wno-availability -Wno-nullability-completeness"
)

// Kclipper provides CI/CD functions for kclipper. Create instances with [New].
type Kclipper struct {
	// Project source directory.
	Source *dagger.Directory
	// Container image registry address (e.g. "ghcr.io/macropower/kclipper").
	Registry string
	// Directory containing only go.mod and go.sum, synced independently of
	// [Kclipper.Source] so that its content hash changes only when dependency
	// files change.
	GoMod *dagger.Directory // +private
	// Go toolchain module instance for delegation.
	Go *dagger.Go // +private
}

// New creates a [Kclipper] module with the given project source directory.
func New(
	// Project source directory.
	// +defaultPath="/"
	source *dagger.Directory,
	// Go module files (go.mod and go.sum only). Synced separately from
	// source so that the go mod download layer is cached independently
	// of source code changes.
	// +defaultPath="/"
	// +ignore=["*", "!go.mod", "!go.sum"]
	goMod *dagger.Directory,
	// Container image registry address.
	// +optional
	registry string,
) *Kclipper {
	if registry == "" {
		registry = defaultRegistry
	}
	return &Kclipper{
		Source:   source,
		GoMod:    goMod,
		Registry: registry,
		Go: dag.Go(dagger.GoOpts{
			Source:   source,
			GoMod:    goMod,
			Registry: registry,
			Cgo:      true,
		}),
	}
}

// devToolchain returns the configured [Dev] toolchain module instance for delegation.
func (m *Kclipper) devToolchain() *dagger.Dev {
	return dag.Dev(dagger.DevOpts{Source: m.Source})
}

// Binary compiles the kcl binary for the given platform.
func (m *Kclipper) Binary(
	// Target build platform.
	// +optional
	platform dagger.Platform,
) *dagger.File {
	return m.Go.Binary("./cmd/kclipper", dagger.GoBinaryOpts{
		NoSymbols: true,
		NoDwarf:   true,
		Platform:  platform,
	})
}

// LintReleaser validates the GoReleaser configuration. Uses
// [Go.GoreleaserCheckBase] with the kclipper remote URL because the
// goreleaser config references a git remote for homebrew/nix repository
// resolution.
//
// +check
func (m *Kclipper) LintReleaser(ctx context.Context) error {
	_, err := m.Go.GoreleaserCheckBase(dagger.GoGoreleaserCheckBaseOpts{
		RemoteURL: kclipperCloneURL,
	}).
		WithExec([]string{"goreleaser", "check"}).
		Sync(ctx)
	return err
}
