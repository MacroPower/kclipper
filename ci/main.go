// CI functions specific to the kclipper repository. The quality gates that run
// local tools (go, golangci-lint, prettier) are Taskfile targets; these
// functions run those same tasks inside the project's devbox environment via
// the devbox toolchain, so CI reproduces exactly what developers run locally:
// local skips the container for speed, CI keeps it for reproducibility.
//
// The rest compose a sibling toolchain directly because their tools are not on
// the devbox PATH: LintActions runs the zizmor toolchain, Security runs the
// security toolchain (Trivy), and the release pipeline (build.go, publish.go)
// runs the goreleaser toolchain -- including its folded-in cosign signing and
// syft SBOM helpers. The release pipeline is intricate: it cross-compiles a
// CGO binary for linux/darwin x amd64/arm64 with a Zig toolchain, a
// pre-downloaded KCL language server per platform, and a macOS SDK fetched from
// the NixOS binary cache, then bundles KCL modules and notarizes the macOS
// binaries. Renovate-config validation stays self-contained here (a pinned
// renovate-config-validator in a Node container) because it is the one check
// neither devbox nor a shared toolchain provides.
package main

import (
	"context"

	"dagger/ci/internal/dagger"
)

const (
	goreleaserVersion = "v2.16.0" // renovate: datasource=github-releases depName=goreleaser/goreleaser
	zigVersion        = "0.15.2"  // renovate: datasource=github-releases depName=ziglang/zig
	kclLSPVersion     = "v0.11.2" // renovate: datasource=github-releases depName=kcl-lang/kcl
	// Base images are pulled from GHCR and ECR Public rather than Docker Hub
	// to avoid anonymous pull rate limits.
	nixImage    = "ghcr.io/nixos/nix:2.34.7"                      // renovate: datasource=docker depName=ghcr.io/nixos/nix
	debianImage = "public.ecr.aws/docker/library/debian:13-slim" // renovate: datasource=docker depName=public.ecr.aws/docker/library/debian

	// macosSDKStorePath is the pinned nixpkgs apple-sdk store path, substituted
	// from cache.nixos.org by [macosSDKDirectory]. Version 15.5 matches the
	// SDK the project previously vendored; bumping it is a deliberate SDK
	// upgrade. Update with: nix eval --raw nixpkgs#apple-sdk_15.outPath (on
	// aarch64-darwin), or look up the current path on search.nixos.org.
	macosSDKStorePath = "/nix/store/92md59ddfbvm6jbxjylgyyg3b9f8kr8n-apple-sdk-15.5"

	kclipperCacheNamespace = "github.com/macropower/kclipper/ci"

	defaultRegistry = "ghcr.io/macropower/kclipper"

	// kclModuleToolPlatform is the platform used for the kcl binary that
	// packages and publishes the KCL modules (LintKCLModules, PublishKCLModules).
	// It is pinned so the built binary's architecture always matches the
	// runtime container it runs in, independent of the engine's native arch.
	kclModuleToolPlatform = dagger.Platform("linux/amd64")

	kclipperCloneURL = "https://github.com/macropower/kclipper.git"

	// macosSDKFlags are the common compiler flags for macOS cross-compilation
	// via Zig, pointing to the macOS SDK headers fetched from the NixOS binary
	// cache (see [macosSDKDirectory]).
	macosSDKFlags = "-F/sdk/MacOSX.sdk/System/Library/Frameworks " +
		"-I/sdk/MacOSX.sdk/usr/include " +
		"-L/sdk/MacOSX.sdk/usr/lib " +
		"-Wno-availability -Wno-nullability-completeness"

	// renovateConfig is the Renovate configuration file validated by
	// [Ci.LintRenovate], relative to the source root.
	renovateConfig = ".github/renovate.json5"

	// Docker Official Image, pulled from Docker's verified publisher
	// space on ECR Public to avoid Docker Hub pull rate limits.
	renovateImage   = "public.ecr.aws/docker/library/node:24-slim" // renovate: datasource=docker depName=public.ecr.aws/docker/library/node
	renovateVersion = "43.224.0"                                   // renovate: datasource=npm depName=renovate

	// zizmorConfig is the zizmor configuration file used by [Ci.LintActions],
	// relative to the source root.
	zizmorConfig = ".github/zizmor.yaml"

	// devboxHome is the home directory of the devbox image's non-root user,
	// under which the Go and golangci-lint caches are mounted.
	devboxHome = "/home/devbox"
	// devboxUser owns the mounted caches so the containerized tasks can
	// write to them.
	devboxUser = "devbox"
	// devboxWorkdir is where the devbox toolchain mounts the project source and
	// runs tasks; the synthetic .git/HEAD is injected here.
	devboxWorkdir = "/src"
)

// Ci provides CI functions for the kclipper repository. Create instances with
// [New].
type Ci struct {
	// Project source directory.
	Source *dagger.Directory
	// Container image registry address (e.g. "ghcr.io/macropower/kclipper").
	Registry string
	// Directory containing only go.mod and go.sum, synced independently of
	// [Ci.Source] so that its content hash changes only when dependency files
	// change.
	GoMod *dagger.Directory // +private
	// Devbox toolchain instance the task-based checks run inside.
	Devbox *dagger.Devbox // +private
	// Goreleaser toolchain used to build, validate, sign, and release the
	// binaries, including its folded-in cosign signing and syft SBOM helpers
	// (see build.go, publish.go).
	Goreleaser *dagger.Goreleaser // +private
	// Scanner is the security toolchain (Trivy) backing [Ci.Security]. Named
	// Scanner rather than Security to avoid colliding with that method.
	Scanner *dagger.Security // +private
	// Zizmor is the zizmor toolchain backing [Ci.LintActions].
	Zizmor *dagger.Zizmor // +private
}

// New creates an [Ci] module with the given project source directory.
func New(
	// Project source directory. Ignore patterns (e.g. .git, dist) belong in the
	// root dagger.json customizations, not here.
	// +defaultPath="/"
	source *dagger.Directory,
	// Go module files (go.mod and go.sum only). Synced separately from source so
	// that the go mod download layer is cached independently of source code
	// changes.
	// +defaultPath="/"
	// +ignore=["*", "!go.mod", "!go.sum"]
	goMod *dagger.Directory,
	// Container image registry address.
	// +optional
	registry string,
) *Ci {
	if registry == "" {
		registry = defaultRegistry
	}
	return &Ci{
		Source:   source,
		GoMod:    goMod,
		Registry: registry,
		Devbox: dag.Devbox(dagger.DevboxOpts{
			Source:         source,
			CacheNamespace: kclipperCacheNamespace,
		}),
		Goreleaser: dag.Goreleaser(dagger.GoreleaserOpts{
			Source:    source,
			Version:   goreleaserVersion,
			RemoteURL: kclipperCloneURL,
		}),
		Scanner: dag.Security(dagger.SecurityOpts{
			Source:         source,
			CacheNamespace: kclipperCacheNamespace + ":security",
		}),
		Zizmor: dag.Zizmor(dagger.ZizmorOpts{
			Source:     source,
			ConfigPath: zizmorConfig,
		}),
	}
}

// env returns the devbox environment container with the project source
// overlaid and the Go module, build, and golangci-lint caches mounted, ready
// to run `devbox run -- task <target>`. The caches persist across runs so the
// containerized tasks reuse work the way the local toolchain does.
//
// The source is mounted without git state (the root dagger.json ignores .git),
// but some tests resolve the repository root via .git/HEAD, so a synthetic HEAD
// is injected at the workdir to keep paths.FindRepoRoot working in the
// container.
func (m *Ci) env() *dagger.Container {
	owner := dagger.ContainerWithMountedCacheOpts{Owner: devboxUser}
	return m.Devbox.WithSource().
		WithNewFile(devboxWorkdir+"/.git/HEAD", "ref: refs/heads/main\n").
		WithMountedCache(devboxHome+"/go/pkg/mod", dag.CacheVolume(kclipperCacheNamespace+":gomod"), owner).
		WithEnvVariable("GOMODCACHE", devboxHome+"/go/pkg/mod").
		WithMountedCache(devboxHome+"/.cache/go-build", dag.CacheVolume(kclipperCacheNamespace+":gobuild"), owner).
		WithEnvVariable("GOCACHE", devboxHome+"/.cache/go-build").
		WithMountedCache(devboxHome+"/.cache/golangci-lint", dag.CacheVolume(kclipperCacheNamespace+":golangci-lint"), owner)
}

// runTask runs a Taskfile target inside the devbox environment, failing if it
// exits non-zero.
func (m *Ci) runTask(ctx context.Context, target string) error {
	_, err := m.env().
		WithExec([]string{"devbox", "run", "--", "task", target}).
		Sync(ctx)
	return err
}

// Binary compiles the kcl binary for the given platform via GoReleaser in
// snapshot mode. There is no longer a lightweight Go toolchain to delegate to,
// so this routes through the full release toolchain (releaserBase): the binary
// is the CGO + Zig cross-compiled artifact the release pipeline produces.
func (m *Ci) Binary(
	ctx context.Context,
	// Target build platform (e.g. "darwin/arm64"). Defaults to linux/amd64.
	// +optional
	platform dagger.Platform,
) (*dagger.File, error) {
	if platform == "" {
		platform = dagger.Platform("linux/amd64")
	}
	return m.BinarySnapshot(ctx, platform)
}
