package main

import (
	"context"
	"time"

	"dagger/kclipper/internal/dagger"
)

// Build runs GoReleaser in snapshot mode, producing binaries for all
// platforms. Returns the dist/ directory. Source archives are skipped in
// snapshot mode since they are only needed for releases.
func (m *Kclipper) Build(ctx context.Context) (*dagger.Directory, error) {
	ctr, err := m.releaserBase(ctx)
	if err != nil {
		return nil, err
	}
	return ctr.
		WithExec([]string{
			"goreleaser", "release", "--snapshot", "--clean",
			"--skip=docker,homebrew,nix,sign,sbom",
			"--parallelism=0",
		}).
		Directory("/src/dist"), nil
}

// BuildImages builds multi-arch runtime container images from a GoReleaser
// dist directory. If no dist is provided, a snapshot build is run.
func (m *Kclipper) BuildImages(
	ctx context.Context,
	// Version label for OCI metadata.
	// +default="snapshot"
	version string,
	// Pre-built GoReleaser dist directory. If not provided, runs a snapshot build.
	// +optional
	dist *dagger.Directory,
) ([]*dagger.Container, error) {
	if dist == nil {
		var err error
		dist, err = m.Build(ctx)
		if err != nil {
			return nil, err
		}
	}
	return runtimeImages(dist, version), nil
}

// runtimeImages builds a multi-arch set of runtime container images from a
// pre-built GoReleaser dist/ directory. Each image is based on debian:13-slim
// with OCI labels, KCL environment variables, and runtime dependencies.
func runtimeImages(dist *dagger.Directory, version string) []*dagger.Container {
	platforms := []dagger.Platform{"linux/amd64", "linux/arm64"}
	variants := make([]*dagger.Container, 0, len(platforms))
	created := time.Now().UTC().Format(time.RFC3339)

	for _, platform := range platforms {
		// Map platform to GoReleaser dist binary path.
		dir := "kclipper_linux_amd64_v1"
		if platform == "linux/arm64" {
			dir = "kclipper_linux_arm64_v8.0"
		}

		ctr := runtimeBase(platform).
			// OCI labels (container config) for metadata.
			WithLabel("org.opencontainers.image.version", version).
			WithLabel("org.opencontainers.image.created", created).
			// OCI annotations (manifest-level) for registry discoverability.
			WithAnnotation("org.opencontainers.image.version", version).
			WithAnnotation("org.opencontainers.image.created", created).
			WithFile("/usr/local/bin/kcl", dist.File(dir+"/kcl")).
			WithEntrypoint([]string{"kcl"})

		variants = append(variants, ctr)
	}

	return variants
}

// runtimeBase returns a debian:13-slim container for the given platform with
// runtime dependencies, OCI labels, and KCL environment variables
// pre-configured.
func runtimeBase(platform dagger.Platform) *dagger.Container {
	return dag.Container(dagger.ContainerOpts{Platform: platform}).
		From("debian:13-slim").
		// Static OCI labels (container config) for metadata.
		WithLabel("org.opencontainers.image.title", "kclipper").
		WithLabel("org.opencontainers.image.description", "A superset of KCL that integrates Helm chart management").
		WithLabel("org.opencontainers.image.source", "https://github.com/macropower/kclipper").
		WithLabel("org.opencontainers.image.url", "https://github.com/macropower/kclipper").
		WithLabel("org.opencontainers.image.licenses", "Apache-2.0").
		// Static OCI annotations (manifest-level) for registry discoverability.
		WithAnnotation("org.opencontainers.image.title", "kclipper").
		WithAnnotation("org.opencontainers.image.source", "https://github.com/macropower/kclipper").
		// KCL environment variables.
		WithEnvVariable("LANG", "en_US.utf8").
		WithEnvVariable("XDG_CACHE_HOME", "/tmp/xdg_cache").
		WithEnvVariable("KCL_LIB_HOME", "/tmp/kcl_lib").
		WithEnvVariable("KCL_PKG_PATH", "/tmp/kcl_pkg").
		WithEnvVariable("KCL_CACHE_PATH", "/tmp/kcl_cache").
		WithEnvVariable("KCL_FAST_EVAL", "1").
		// Install runtime dependencies (curl/gpg for plugin installs).
		WithExec([]string{"sh", "-c",
			"apt-get update && apt-get install -y curl gpg apt-transport-https && rm -rf /var/lib/apt/lists/* /tmp/*"})
}

// releaserBase returns a container with Go, GoReleaser, cosign, syft, Zig,
// pre-downloaded KCL Language Server binaries, and macOS SDK headers needed
// for CGO cross-compilation. Provides the complete release toolset for
// goreleaser release with signing, SBOM, and cross-compilation support.
func (m *Kclipper) releaserBase(ctx context.Context) (*dagger.Container, error) {
	goVersion, err := m.Go.Version(ctx)
	if err != nil {
		return nil, err
	}
	ctr := dag.Container().
		From("golang:"+goVersion).
		WithFile("/usr/local/bin/goreleaser",
			dag.Container().From("ghcr.io/goreleaser/goreleaser:"+goreleaserVersion).
				File("/usr/bin/goreleaser")).
		WithFile("/usr/local/bin/cosign",
			dag.Container().From("gcr.io/projectsigstore/cosign:"+cosignVersion).
				File("/ko-app/cosign")).
		WithFile("/usr/local/bin/syft",
			dag.Container().From("ghcr.io/anchore/syft:"+syftVersion).
				File("/syft")).
		WithMountedCache("/go/pkg/mod", m.Go.ModuleCache()).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", m.Go.BuildCache()).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithDirectory("/src", m.GoMod).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"}).
		WithMountedDirectory("/src", m.Source)
	ctr = m.Go.EnsureGitRepo(ctr, dagger.GoEnsureGitRepoOpts{
		RemoteURL: kclipperCloneURL,
	})
	return ctr.
		// Install Zig for CGO cross-compilation.
		WithExec([]string{
			"sh", "-c",
			"apt-get update && apt-get install -y xz-utils && " +
				"ZIG_ARCH=$(uname -m | sed 's/arm64/aarch64/') && " +
				"curl -fsSL https://ziglang.org/download/" + zigVersion +
				"/zig-${ZIG_ARCH}-linux-" + zigVersion + ".tar.xz | " +
				"tar -xJ -C /usr/local --strip-components=1 && " +
				"ln -sf /usr/local/zig /usr/local/bin/zig",
		}).
		// Pre-download KCL Language Server for all target platforms.
		WithExec([]string{
			"sh", "-c",
			"for pair in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do " +
				"os=${pair%%/*} && arch=${pair##*/} && " +
				"mkdir -p /lsp/${os}/${arch} && " +
				"curl -fsSL https://github.com/kcl-lang/kcl/releases/download/" + kclLSPVersion +
				"/kclvm-" + kclLSPVersion + "-${os}-${arch}.tar.gz | " +
				"tar -xz --strip-components=2 -C /lsp/${os}/${arch} kclvm/bin/kcl-language-server; " +
				"done",
		}).
		// Mount macOS SDK headers for Darwin cross-compilation.
		WithMountedDirectory("/sdk/MacOSX.sdk",
			m.Source.Directory(".nixpkgs/vendor/Platforms/MacOSX.platform/Developer/SDKs/MacOSX.sdk")).
		WithEnvVariable("SDK_PATH", "/sdk/MacOSX.sdk").
		// Env vars used by GoReleaser ldflags and templates.
		WithEnvVariable("KCL_LSP_VERSION", kclLSPVersion).
		WithEnvVariable("BUILD_TAGS", "netgo").
		WithEnvVariable("HOSTNAME", "dagger").
		WithEnvVariable("USER", "dagger").
		// CC/CXX env vars for GoReleaser cross-compilation via Zig.
		WithEnvVariable("CC_LINUX_AMD64", "/src/hack/zig-gold-wrapper.sh -target x86_64-linux-gnu").
		WithEnvVariable("CC_LINUX_ARM64", "/src/hack/zig-gold-wrapper.sh -target aarch64-linux-gnu").
		WithEnvVariable("CC_DARWIN_AMD64", "/src/hack/zig-macos-wrapper.sh -target x86_64-macos-none "+macosSDKFlags).
		WithEnvVariable("CC_DARWIN_ARM64", "/src/hack/zig-macos-wrapper.sh -target aarch64-macos-none "+macosSDKFlags).
		WithEnvVariable("CXX_LINUX_AMD64", "/src/hack/zig-gold-wrapper.sh -target x86_64-linux-gnu").
		WithEnvVariable("CXX_LINUX_ARM64", "/src/hack/zig-gold-wrapper.sh -target aarch64-linux-gnu").
		WithEnvVariable("CXX_DARWIN_AMD64", "/src/hack/zig-macos-wrapper.sh -target x86_64-macos-none "+macosSDKFlags).
		WithEnvVariable("CXX_DARWIN_ARM64", "/src/hack/zig-macos-wrapper.sh -target aarch64-macos-none "+macosSDKFlags), nil
}
