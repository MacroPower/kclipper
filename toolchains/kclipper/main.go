// CI/CD functions specific to the kclipper project. Provides building,
// releasing, linting non-Go files, benchmarking, and development container
// support. Generic Go CI functions (testing, Go linting, Go formatting) are
// provided by the [Go] toolchain module; this module adds kclipper-specific
// logic and project-level tooling (prettier, zizmor, goreleaser, cosign,
// syft, deadcode).

package main

import (
	"context"

	"dagger/kclipper/internal/dagger"
)

const (
	goreleaserVersion = "v2.13.3"  // renovate: datasource=github-releases depName=goreleaser/goreleaser
	prettierVersion   = "3.5.3"    // renovate: datasource=npm depName=prettier
	zizmorVersion     = "1.22.0"   // renovate: datasource=github-releases depName=zizmorcore/zizmor
	deadcodeVersion   = "v0.42.0"  // renovate: datasource=go depName=golang.org/x/tools
	cosignVersion     = "v3.0.4"   // renovate: datasource=github-releases depName=sigstore/cosign
	syftVersion       = "v1.41.1"  // renovate: datasource=github-releases depName=anchore/syft
	zigVersion        = "0.15.2"   // renovate: datasource=github-releases depName=ziglang/zig
	kclLSPVersion     = "v0.11.2"  // renovate: datasource=github-releases depName=kcl-lang/kcl

	kclipperCacheNamespace = "github.com/macropower/kclipper/toolchains/kclipper"

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
			Source: source,
			GoMod:  goMod,
			Cgo:    true,
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

// ---------------------------------------------------------------------------
// Base containers (private)
// ---------------------------------------------------------------------------

// prettierBase returns a Node container with prettier pre-installed.
// Callers must mount their source directory and set the workdir.
func (m *Kclipper) prettierBase() *dagger.Container {
	return dag.Container().
		From("node:lts-slim").
		WithMountedCache("/root/.npm", dag.CacheVolume(kclipperCacheNamespace+":npm")).
		WithExec([]string{"npm", "install", "-g", "prettier@" + prettierVersion})
}

// goreleaserCheckBase extends [Kclipper.goreleaserBase] with a minimal git
// repo (via [Go.EnsureGitRepo]) for the given remoteURL. This is sufficient
// for goreleaser check, which only validates config syntax and does not
// require the full release toolset provided by [Kclipper.releaserBase].
func (m *Kclipper) goreleaserCheckBase(ctx context.Context, remoteURL string) (*dagger.Container, error) {
	ctr, err := m.goreleaserBase(ctx)
	if err != nil {
		return nil, err
	}
	ctr = ctr.WithMountedDirectory("/src", m.Source)
	return m.Go.EnsureGitRepo(ctr, dagger.GoEnsureGitRepoOpts{
		RemoteURL: remoteURL,
	}), nil
}

// defaultPrettierPatterns returns the default file patterns for prettier
// formatting and linting.
func defaultPrettierPatterns() []string {
	return []string{
		"*.yaml", "*.md", "*.json",
		"**/*.yaml", "**/*.md", "**/*.json",
	}
}
