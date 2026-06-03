// CI/CD functions specific to the kclipper project. Provides building,
// releasing, linting non-Go files, benchmarking, and development container
// support. Generic Go CI functions (testing, Go linting, Go formatting) are
// provided by the [Go] toolchain module; this module adds kclipper-specific
// logic and project-level tooling (prettier, zizmor, goreleaser, cosign,
// syft, deadcode).

package main

import (
	"dagger/kclipper/internal/dagger"
)

const (
	goreleaserVersion = "v2.13.3" // renovate: datasource=github-releases depName=goreleaser/goreleaser
	prettierVersion   = "3.5.3"   // renovate: datasource=npm depName=prettier
	cosignVersion     = "v3.0.4"  // renovate: datasource=github-releases depName=sigstore/cosign
	syftVersion       = "v1.41.1" // renovate: datasource=github-releases depName=anchore/syft
	zigVersion        = "0.15.2"  // renovate: datasource=github-releases depName=ziglang/zig
	kclLSPVersion     = "v0.11.2" // renovate: datasource=github-releases depName=kcl-lang/kcl

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
	// GoReleaser toolchain module instance for config validation.
	Goreleaser *dagger.Goreleaser // +private
	// Zizmor toolchain module instance for GitHub Actions linting.
	Zizmor *dagger.Zizmor // +private
	// Cosign toolchain module instance for container image signing.
	Cosign *dagger.Cosign // +private
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
	goToolchain := dag.Go(dagger.GoOpts{
		Source:         source,
		GoMod:          goMod,
		Cgo:            true,
		Version:        "1.25",
		CacheNamespace: "github.com/macropower/kclipper/toolchains/go",
	})
	return &Kclipper{
		Source:   source,
		GoMod:    goMod,
		Registry: registry,
		Go:       goToolchain,
		Goreleaser: dag.Goreleaser(dagger.GoreleaserOpts{
			Source:    source,
			Base:      goToolchain.Base(),
			Version:   goreleaserVersion,
			RemoteURL: kclipperCloneURL,
		}),
		Zizmor: dag.Zizmor(dagger.ZizmorOpts{Source: source}),
		Cosign: dag.Cosign(),
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

// defaultPrettierPatterns returns the default file patterns for prettier
// formatting and linting.
func defaultPrettierPatterns() []string {
	return []string{
		"*.yaml", "*.md", "*.json",
		"**/*.yaml", "**/*.md", "**/*.json",
	}
}
