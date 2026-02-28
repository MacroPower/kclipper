package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
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
	return runtimeImages(ctx, dist, version)
}

// runtimeImages builds a multi-arch set of runtime container images from a
// pre-built GoReleaser dist/ directory. Each image is based on debian:13-slim
// with OCI labels, KCL environment variables, and runtime dependencies.
func runtimeImages(_ context.Context, dist *dagger.Directory, version string) ([]*dagger.Container, error) {
	platforms := []dagger.Platform{"linux/amd64", "linux/arm64"}
	variants := make([]*dagger.Container, len(platforms))
	created := time.Now().UTC().Format(time.RFC3339)

	for i, platform := range platforms {
		// Map platform to GoReleaser dist binary path.
		dir := "kclipper_linux_amd64_v1"
		if platform == "linux/arm64" {
			dir = "kclipper_linux_arm64_v8.0"
		}

		variants[i] = runtimeBase(platform).
			// OCI labels (container config) for metadata.
			WithLabel("org.opencontainers.image.version", version).
			WithLabel("org.opencontainers.image.created", created).
			// OCI annotations (manifest-level) for registry discoverability.
			WithAnnotation("org.opencontainers.image.version", version).
			WithAnnotation("org.opencontainers.image.created", created).
			WithFile("/usr/local/bin/kcl", dist.File(dir+"/kcl")).
			WithEntrypoint([]string{"kcl"})
	}

	return variants, nil
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

// verifyBinaryPlatform runs the `file` command on a built binary and asserts
// that the reported architecture matches the expected architecture for the
// given platform. Returns an error if the architecture string is absent from
// the output, indicating a cross-compilation mismatch.
func verifyBinaryPlatform(ctx context.Context, bin *dagger.File, platform dagger.Platform) error {
	name, err := bin.Name(ctx)
	if err != nil {
		return fmt.Errorf("get binary name: %w", err)
	}

	arch := filepath.Base(strings.SplitN(string(platform), "/", 2)[1])
	expected, ok := platformToFileArch[arch]
	if !ok {
		return fmt.Errorf("unknown platform architecture %q", arch)
	}

	mntPath := filepath.Join("/mnt", name)
	out, err := dag.Container().
		From("debian:13-slim").
		WithExec([]string{"sh", "-c", "apt-get update -qq && apt-get install -y -qq file"}).
		WithMountedFile(mntPath, bin).
		WithExec([]string{"file", mntPath}).
		Stdout(ctx)
	if err != nil {
		return fmt.Errorf("run file on binary %s: %w", name, err)
	}

	if !strings.Contains(out, expected) {
		return fmt.Errorf("binary %s: expected architecture %q (%s) not found in file output: %s", name, expected, arch, out)
	}

	return nil
}

// platformToFileArch maps a Go platform architecture name to the architecture
// string produced by the `file` command.
var platformToFileArch = map[string]string{
	"amd64": "x86-64",
	"arm64": "aarch64",
}

// goreleaserBase returns a container with Go, GoReleaser, module caches, and
// the project source pre-downloaded. This is the common base shared by
// [Kclipper.releaserBase] and [Kclipper.goreleaserCheckBase]. Callers are
// responsible for initializing a git repo (e.g. via [Go.EnsureGitRepo]) with
// their appropriate remote URL before use.
//
// The container is built on top of [Go.Base], reusing the pre-built Go image
// with module cache and go mod download already completed.
func (m *Kclipper) goreleaserBase(_ context.Context) (*dagger.Container, error) {
	return m.Go.Base().
		WithFile("/usr/local/bin/goreleaser",
			dag.Container().From("ghcr.io/goreleaser/goreleaser:"+goreleaserVersion).
				File("/usr/bin/goreleaser")).
		WithMountedDirectory("/src", m.Source), nil
}

// zigDirectory returns the Zig compiler distribution directory for the host
// platform, extracted from the official tarball in a dedicated container for
// independent caching.
func zigDirectory() *dagger.Directory {
	return dag.Container().
		From("debian:13-slim").
		WithExec([]string{"sh", "-c",
			"apt-get update && apt-get install -y xz-utils curl && " +
				"ZIG_ARCH=$(uname -m | sed 's/arm64/aarch64/') && " +
				"curl -fsSL https://ziglang.org/download/" + zigVersion +
				"/zig-${ZIG_ARCH}-linux-" + zigVersion + ".tar.xz | " +
				"tar -xJ -C /opt --strip-components=1"}).
		Directory("/opt")
}

// kclLSPBinary returns the KCL Language Server binary for the given os/arch
// combination, downloaded in a dedicated container for independent caching per
// platform.
func kclLSPBinary(goos, goarch string) *dagger.File {
	return dag.Container().
		From("debian:13-slim").
		WithExec([]string{"sh", "-c",
			"apt-get update && apt-get install -y curl && " +
				"curl -fsSL https://github.com/kcl-lang/kcl/releases/download/" + kclLSPVersion +
				"/kclvm-" + kclLSPVersion + "-" + goos + "-" + goarch + ".tar.gz | " +
				"tar -xz --strip-components=2 kclvm/bin/kcl-language-server"}).
		File("/kcl-language-server")
}

// releaserBase extends [Kclipper.goreleaserBase] with cosign, syft, Zig,
// pre-downloaded KCL Language Server binaries, and macOS SDK headers needed
// for CGO cross-compilation. Provides the complete release toolset for
// goreleaser release with signing, SBOM, and cross-compilation support.
func (m *Kclipper) releaserBase(ctx context.Context) (*dagger.Container, error) {
	ctr, err := m.goreleaserBase(ctx)
	if err != nil {
		return nil, err
	}
	// Install stable tools before committing source so that source changes
	// only invalidate layers from EnsureGitRepo onward, not the tool layers.
	ctr = ctr.
		WithFile("/usr/local/bin/cosign",
			dag.Container().From("gcr.io/projectsigstore/cosign:"+cosignVersion).
				File("/ko-app/cosign")).
		WithFile("/usr/local/bin/syft",
			dag.Container().From("ghcr.io/anchore/syft:"+syftVersion).
				File("/syft")).
		// Install Zig for CGO cross-compilation from a dedicated cached container.
		WithDirectory("/usr/local", zigDirectory()).
		WithExec([]string{"ln", "-sf", "/usr/local/zig", "/usr/local/bin/zig"}).
		// Pre-download KCL Language Server for all target platforms; each platform
		// is fetched in an independent container so Dagger caches them separately
		// and evaluates them concurrently.
		WithFile("/lsp/linux/amd64/kcl-language-server", kclLSPBinary("linux", "amd64")).
		WithFile("/lsp/linux/arm64/kcl-language-server", kclLSPBinary("linux", "arm64")).
		WithFile("/lsp/darwin/amd64/kcl-language-server", kclLSPBinary("darwin", "amd64")).
		WithFile("/lsp/darwin/arm64/kcl-language-server", kclLSPBinary("darwin", "arm64")).
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
		WithEnvVariable("CXX_DARWIN_ARM64", "/src/hack/zig-macos-wrapper.sh -target aarch64-macos-none "+macosSDKFlags)
	// Commit source last so that source changes only invalidate layers from
	// here onward, preserving the tool installation layers above.
	ctr = m.Go.EnsureGitRepo(ctr, dagger.GoEnsureGitRepoOpts{
		RemoteURL: kclipperCloneURL,
	})
	// Mount macOS SDK headers for Darwin cross-compilation.
	return ctr.WithMountedDirectory("/sdk/MacOSX.sdk",
		m.Source.Directory(".nixpkgs/vendor/Platforms/MacOSX.platform/Developer/SDKs/MacOSX.sdk")).
		WithEnvVariable("SDK_PATH", "/sdk/MacOSX.sdk"), nil
}
