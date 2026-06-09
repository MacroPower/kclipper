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
	goreleaserVersion = "v2.16.0" // renovate: datasource=github-releases depName=goreleaser/goreleaser
	zigVersion        = "0.15.2"  // renovate: datasource=github-releases depName=ziglang/zig
	kclLSPVersion     = "v0.11.2" // renovate: datasource=github-releases depName=kcl-lang/kcl
	// Base images are pulled from GHCR and ECR Public rather than Docker Hub
	// to avoid anonymous pull rate limits.
	nixImage    = "ghcr.io/nixos/nix:2.34.7"                     // renovate: datasource=docker depName=ghcr.io/nixos/nix
	debianImage = "public.ecr.aws/docker/library/debian:13-slim" // renovate: datasource=docker depName=public.ecr.aws/docker/library/debian

	// macosSDKStorePath is the pinned nixpkgs apple-sdk store path, substituted
	// from cache.nixos.org by [macosSDKDirectory]. Version 15.5 matches the
	// SDK the project previously vendored; bumping it is a deliberate SDK
	// upgrade. Update with: nix eval --raw nixpkgs#apple-sdk_15.outPath (on
	// aarch64-darwin), or look up the current path on search.nixos.org.
	macosSDKStorePath = "/nix/store/92md59ddfbvm6jbxjylgyyg3b9f8kr8n-apple-sdk-15.5"

	kclipperCacheNamespace = "github.com/macropower/kclipper/toolchains/kclipper"

	defaultRegistry = "ghcr.io/macropower/kclipper"

	kclipperCloneURL = "https://github.com/macropower/kclipper.git"

	// macosSDKFlags are the common compiler flags for macOS cross-compilation
	// via Zig, pointing to the macOS SDK headers fetched from the NixOS binary
	// cache (see [macosSDKDirectory]).
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
	// Prettier toolchain module instance for non-Go formatting and linting.
	Prettier *dagger.Prettier // +private
	// Syft toolchain module instance for SBOM binary installation.
	Syft *dagger.Syft // +private
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
		Version:        "1.26",
		CacheNamespace: "github.com/macropower/kclipper/toolchains/go",
		// Tests resolve the repository root via .git/HEAD, but source is
		// synced without git state; inject a synthetic .git/HEAD so
		// paths.FindRepoRoot works inside containers.
		InjectGitHead: true,
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
		Prettier: dag.Prettier(dagger.PrettierOpts{
			Source:         source,
			CacheNamespace: kclipperCacheNamespace,
		}),
		Syft: dag.Syft(),
	}
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
