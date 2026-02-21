package main

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"

	"dagger/ci/internal/dagger"
)

const (
	goVersion           = "1.25"           // renovate: datasource=golang-version depName=go
	golangciLintVersion = "v2.9"           // renovate: datasource=github-releases depName=golangci/golangci-lint
	goreleaserVersion   = "v2.13.3"        // renovate: datasource=github-releases depName=goreleaser/goreleaser
	zigVersion          = "0.15.2"         // renovate: datasource=github-releases depName=ziglang/zig
	cosignVersion       = "v3.0.4"         // renovate: datasource=github-releases depName=sigstore/cosign
	syftVersion         = "v1.41.1"        // renovate: datasource=github-releases depName=anchore/syft
	prettierVersion     = "3.5.3"          // renovate: datasource=npm depName=prettier
	zizmorVersion       = "1.22.0"         // renovate: datasource=github-releases depName=zizmorcore/zizmor
	kclLSPVersion       = "v0.11.2"        // renovate: datasource=github-releases depName=kcl-lang/kcl

	imageRegistry = "ghcr.io/macropower/kclipper"
)

// Ci provides CI/CD functions for kclipper.
type Ci struct {
	// Project source directory.
	Source *dagger.Directory
}

// New creates a Ci module with the given project source directory.
func New(
	// Project source directory.
	// +defaultPath="/"
	source *dagger.Directory,
) *Ci {
	return &Ci{Source: source}
}

// ---------------------------------------------------------------------------
// Testing
// ---------------------------------------------------------------------------

// Test runs Go tests with race detection and vet checking.
//
// +check
func (m *Ci) Test(ctx context.Context) error {
	_, err := m.goBase().
		WithExec([]string{
			"go", "test", "-race", "-vet=all", "./...",
		}).
		Sync(ctx)
	return err
}

// TestCoverage runs Go tests and returns the coverage profile file.
func (m *Ci) TestCoverage() *dagger.File {
	return m.goBase().
		WithExec([]string{
			"go", "test", "-race", "-vet=all",
			"-coverprofile=/tmp/coverage.txt", "./...",
		}).
		File("/tmp/coverage.txt")
}

// ---------------------------------------------------------------------------
// Linting
// ---------------------------------------------------------------------------

// Lint runs golangci-lint on the source code.
//
// +check
func (m *Ci) Lint(ctx context.Context) error {
	_, err := m.lintBase().
		WithExec([]string{"golangci-lint", "run"}).
		Sync(ctx)
	return err
}

// LintPrettier checks YAML, JSON, and Markdown formatting.
//
// +check
func (m *Ci) LintPrettier(ctx context.Context) error {
	_, err := dag.Container().
		From("node:lts-slim").
		WithExec([]string{"npm", "install", "-g", "prettier@" + prettierVersion}).
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec([]string{
			"prettier", "--config", "./.prettierrc.yaml", "--check",
			"*.yaml", "*.md", "*.json",
			"**/*.yaml", "**/*.md", "**/*.json",
		}).
		Sync(ctx)
	return err
}

// LintActions runs zizmor to lint GitHub Actions workflows.
//
// +check
func (m *Ci) LintActions(ctx context.Context) error {
	_, err := dag.Container().
		From("ghcr.io/zizmorcore/zizmor:"+zizmorVersion).
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec([]string{
			"zizmor", ".github/workflows", "--config", ".github/zizmor.yaml",
		}).
		Sync(ctx)
	return err
}

// LintReleaser validates the GoReleaser configuration.
//
// +check
func (m *Ci) LintReleaser(ctx context.Context) error {
	_, err := m.releaserBase().
		WithExec([]string{"goreleaser", "check"}).
		Sync(ctx)
	return err
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

// Format runs golangci-lint --fix and prettier --write, returning the
// modified source directory.
func (m *Ci) Format() *dagger.Directory {
	// Go formatting via golangci-lint --fix.
	goFmt := m.lintBase().
		WithExec([]string{"golangci-lint", "run", "--fix"}).
		Directory("/src")

	// Prettier formatting.
	return dag.Container().
		From("node:lts-slim").
		WithExec([]string{"npm", "install", "-g", "prettier@" + prettierVersion}).
		WithMountedDirectory("/src", goFmt).
		WithWorkdir("/src").
		WithExec([]string{
			"prettier", "--config", "./.prettierrc.yaml", "-w",
			"*.yaml", "*.md", "*.json",
			"**/*.yaml", "**/*.md", "**/*.json",
		}).
		Directory("/src")
}

// ---------------------------------------------------------------------------
// Building
// ---------------------------------------------------------------------------

// Build runs GoReleaser in snapshot mode, producing binaries for all
// platforms. Returns the dist/ directory.
func (m *Ci) Build() *dagger.Directory {
	return m.releaserBase().
		WithExec([]string{
			"goreleaser", "release", "--snapshot", "--clean",
			"--skip=docker,homebrew,nix,sign",
		}).
		Directory("/src/dist")
}

// PublishImages builds multi-arch container images using Dagger's native
// Container API and publishes them to the registry.
//
// Multiple tags are published to match the current GoReleaser behavior:
// :latest, :vX.Y.Z, :vX, :vX.Y.
func (m *Ci) PublishImages(
	ctx context.Context,
	// tags is the list of image tags to publish (e.g. ["latest", "v1.2.3", "v1", "v1.2"]).
	tags []string,
	registryUsername string,
	registryPassword *dagger.Secret,
	// +optional
	cosignKey *dagger.Secret,
) (string, error) {
	// Build binaries via GoReleaser (Docker skipped).
	dist := m.releaserBase().
		WithExec([]string{
			"goreleaser", "release", "--snapshot", "--clean",
			"--skip=docker,homebrew,nix,sign",
		}).
		Directory("/src/dist")

	return m.publishImages(ctx, dist, tags, registryUsername, registryPassword, cosignKey)
}

// publishImages builds multi-arch container images from a pre-built dist/
// directory and publishes them to the registry. This is the internal
// implementation shared by PublishImages() and Release().
func (m *Ci) publishImages(
	ctx context.Context,
	dist *dagger.Directory,
	tags []string,
	registryUsername string,
	registryPassword *dagger.Secret,
	cosignKey *dagger.Secret,
) (string, error) {
	// Build a container image for each platform.
	platforms := []dagger.Platform{"linux/amd64", "linux/arm64"}
	variants := make([]*dagger.Container, 0, len(platforms))

	for _, platform := range platforms {
		// Map platform to GoReleaser dist binary path.
		// GoReleaser uses the build id (kclipper), not the binary name (kcl).
		// Directory names include the GOAMD64/GOARM64 version suffix:
		//   amd64 -> kclipper_linux_amd64_v1
		//   arm64 -> kclipper_linux_arm64_v8.0
		dir := "kclipper_linux_amd64_v1"
		if platform == "linux/arm64" {
			dir = "kclipper_linux_arm64_v8.0"
		}
		binaryPath := dir + "/kcl"

		ctr := dag.Container(dagger.ContainerOpts{Platform: platform}).
			From("debian:13-slim").
			// Match env vars from existing Dockerfile.
			WithEnvVariable("LANG", "en_US.utf8").
			WithEnvVariable("XDG_CACHE_HOME", "/tmp/xdg_cache").
			WithEnvVariable("KCL_LIB_HOME", "/tmp/kcl_lib").
			WithEnvVariable("KCL_PKG_PATH", "/tmp/kcl_pkg").
			WithEnvVariable("KCL_CACHE_PATH", "/tmp/kcl_cache").
			WithEnvVariable("KCL_FAST_EVAL", "1").
			// Install runtime dependencies (curl/gpg for plugin installs).
			WithExec([]string{"sh", "-c",
				"apt-get update && apt-get install -y curl gpg apt-transport-https && rm -rf /var/lib/apt/lists/* /tmp/*"}).
			WithFile("/usr/local/bin/kcl", dist.File(binaryPath)).
			WithEntrypoint([]string{"kcl"})

		variants = append(variants, ctr)
	}

	// Publish multi-arch manifest for each tag and collect digests.
	var digests []string
	for _, t := range tags {
		ref := fmt.Sprintf("%s:%s", imageRegistry, t)
		digest, err := dag.Container().
			WithRegistryAuth("ghcr.io", registryUsername, registryPassword).
			Publish(ctx, ref, dagger.ContainerPublishOpts{
				PlatformVariants: variants,
			})
		if err != nil {
			return "", fmt.Errorf("publish %s: %w", ref, err)
		}
		digests = append(digests, digest)
	}

	// Sign each published image with cosign (key-based signing).
	if cosignKey != nil {
		cosignCtr := dag.Container().
			From("gcr.io/projectsigstore/cosign:" + cosignVersion).
			WithRegistryAuth("ghcr.io", registryUsername, registryPassword).
			WithSecretVariable("COSIGN_KEY", cosignKey)

		for _, digest := range digests {
			_, err := cosignCtr.
				WithExec([]string{"cosign", "sign", "--key", "env://COSIGN_KEY", digest, "--yes"}).
				Stdout(ctx)
			if err != nil {
				return "", fmt.Errorf("sign image %s: %w", digest, err)
			}
		}
	}

	return fmt.Sprintf("published %d tags, %d digests\n%s", len(tags), len(digests), strings.Join(digests, "\n")), nil
}

// Release runs GoReleaser for binaries/archives/signing, then builds and
// publishes container images using Dagger-native Container.Publish().
// GoReleaser's Docker support is skipped entirely to avoid Docker-in-Docker.
//
// Returns the dist/ directory containing checksums.txt and digests.txt
// for attestation in the calling workflow.
func (m *Ci) Release(
	ctx context.Context,
	githubToken *dagger.Secret,
	registryUsername string,
	registryPassword *dagger.Secret,
	tag string,
	// +optional
	cosignKey *dagger.Secret,
	// +optional
	macosSignP12 *dagger.Secret,
	// +optional
	macosSignPassword *dagger.Secret,
	// +optional
	macosNotaryKey *dagger.Secret,
	// +optional
	macosNotaryKeyId *dagger.Secret,
	// +optional
	macosNotaryIssuerId *dagger.Secret,
) (*dagger.Directory, error) {
	ctr := m.releaserBase().
		WithSecretVariable("GITHUB_TOKEN", githubToken)

	// Conditionally add macOS signing secrets.
	if macosSignP12 != nil {
		ctr = ctr.
			WithSecretVariable("MACOS_SIGN_P12", macosSignP12).
			WithSecretVariable("MACOS_SIGN_PASSWORD", macosSignPassword).
			WithSecretVariable("MACOS_NOTARY_KEY", macosNotaryKey).
			WithSecretVariable("MACOS_NOTARY_KEY_ID", macosNotaryKeyId).
			WithSecretVariable("MACOS_NOTARY_ISSUER_ID", macosNotaryIssuerId)
	}

	// Run GoReleaser for binaries, archives, signing, Homebrew, Nix.
	// Docker is skipped -- images are published natively via Dagger below.
	dist := ctr.
		WithExec([]string{"goreleaser", "release", "--clean", "--skip=docker"}).
		Directory("/src/dist")

	// Derive image tags from the version tag (e.g. v1.2.3 -> latest, v1.2.3, v1, v1.2).
	tags := versionTags(tag)

	// Publish multi-arch container images via Dagger-native API.
	// Reuse the dist directory from the goreleaser run to avoid building twice.
	result, err := m.publishImages(ctx, dist, tags, registryUsername, registryPassword, cosignKey)
	if err != nil {
		return nil, fmt.Errorf("publish images: %w", err)
	}

	// Extract digests from the publish result and write to dist for attestation.
	lines := strings.Split(result, "\n")
	var digestLines []string
	for _, line := range lines {
		// Digest lines contain "@sha256:".
		if strings.Contains(line, "@sha256:") {
			digestLines = append(digestLines, line)
		}
	}
	if len(digestLines) > 0 {
		dist = dist.WithNewFile("digests.txt", strings.Join(digestLines, "\n")+"\n")
	}

	return dist, nil
}

// ---------------------------------------------------------------------------
// Composite
// ---------------------------------------------------------------------------

// Validate runs Test, Lint, LintPrettier, LintActions, and LintReleaser
// concurrently. Use this for PR validation.
func (m *Ci) Validate(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error { return m.Test(ctx) })
	eg.Go(func() error { return m.Lint(ctx) })
	eg.Go(func() error { return m.LintPrettier(ctx) })
	eg.Go(func() error { return m.LintActions(ctx) })
	eg.Go(func() error { return m.LintReleaser(ctx) })

	return eg.Wait()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// versionTags derives the set of image tags from a version tag string.
// e.g. "v1.2.3" -> ["latest", "v1.2.3", "v1", "v1.2"].
func versionTags(tag string) []string {
	tags := []string{"latest", tag}
	v := strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 1 {
		tags = append(tags, "v"+parts[0])
	}
	if len(parts) >= 2 {
		tags = append(tags, "v"+parts[0]+"."+parts[1])
	}
	return tags
}

// ---------------------------------------------------------------------------
// Base containers (private helpers)
// ---------------------------------------------------------------------------

// lintBase returns a golangci-lint container with source and caches. It
// installs linux-headers so that CGO transitive dependencies (e.g.
// containers/storage) compile on Alpine.
func (m *Ci) lintBase() *dagger.Container {
	return dag.Container().
		From("golangci/golangci-lint:"+golangciLintVersion+"-alpine").
		WithExec([]string{"apk", "add", "--no-cache", "linux-headers"}).
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithMountedCache("/root/.cache/golangci-lint", dag.CacheVolume("golangci-lint")).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache")
}

// goBase returns a Go container with source, module cache, and build cache.
func (m *Ci) goBase() *dagger.Container {
	return ensureGitRepo(dag.Container().
		From("golang:"+goVersion).
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache"))
}

// releaserBase returns a container with Go, GoReleaser, Zig, cosign, and the
// macOS SDK headers needed for CGO cross-compilation.
func (m *Ci) releaserBase() *dagger.Container {
	return ensureGitRepo(dag.Container().
		From("golang:"+goVersion).
		// Install Zig for CGO cross-compilation, plus jq (used by
		// GoReleaser before.hooks to download KCL LSP).
		WithExec([]string{
			"sh", "-c",
			"apt-get update && apt-get install -y xz-utils jq && " +
				"ZIG_ARCH=$(uname -m | sed 's/arm64/aarch64/') && " +
				"curl -fsSL https://ziglang.org/download/" + zigVersion +
				"/zig-${ZIG_ARCH}-linux-" + zigVersion + ".tar.xz | " +
				"tar -xJ -C /usr/local --strip-components=1 && " +
				"ln -sf /usr/local/zig /usr/local/bin/zig",
		}).
		// Install GoReleaser.
		WithExec([]string{
			"go", "install",
			"github.com/goreleaser/goreleaser/v2@" + goreleaserVersion,
		}).
		// Install cosign for artifact signing.
		WithExec([]string{
			"go", "install",
			"github.com/sigstore/cosign/v3/cmd/cosign@" + cosignVersion,
		}).
		// Install syft for SBOM generation.
		WithExec([]string{
			"go", "install",
			"github.com/anchore/syft/cmd/syft@" + syftVersion,
		}).
		// Mount source and caches.
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache").
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
		WithEnvVariable("CC_DARWIN_AMD64", "/src/hack/zig-macos-wrapper.sh -target x86_64-macos-none -F/sdk/MacOSX.sdk/System/Library/Frameworks -I/sdk/MacOSX.sdk/usr/include -L/sdk/MacOSX.sdk/usr/lib -Wno-availability -Wno-nullability-completeness").
		WithEnvVariable("CC_DARWIN_ARM64", "/src/hack/zig-macos-wrapper.sh -target aarch64-macos-none -F/sdk/MacOSX.sdk/System/Library/Frameworks -I/sdk/MacOSX.sdk/usr/include -L/sdk/MacOSX.sdk/usr/lib -Wno-availability -Wno-nullability-completeness").
		WithEnvVariable("CXX_LINUX_AMD64", "/src/hack/zig-gold-wrapper.sh -target x86_64-linux-gnu").
		WithEnvVariable("CXX_LINUX_ARM64", "/src/hack/zig-gold-wrapper.sh -target aarch64-linux-gnu").
		WithEnvVariable("CXX_DARWIN_AMD64", "/src/hack/zig-macos-wrapper.sh -target x86_64-macos-none -F/sdk/MacOSX.sdk/System/Library/Frameworks -I/sdk/MacOSX.sdk/usr/include -L/sdk/MacOSX.sdk/usr/lib -Wno-availability -Wno-nullability-completeness").
		WithEnvVariable("CXX_DARWIN_ARM64", "/src/hack/zig-macos-wrapper.sh -target aarch64-macos-none -F/sdk/MacOSX.sdk/System/Library/Frameworks -I/sdk/MacOSX.sdk/usr/include -L/sdk/MacOSX.sdk/usr/lib -Wno-availability -Wno-nullability-completeness"))
}

// ensureGitRepo ensures the container has a valid git repository at its
// working directory. When running from a git worktree, the .git file
// references a host path that doesn't exist in the container. In that case,
// a minimal git repository is initialized so that tools like GoReleaser and
// tests that depend on git metadata continue to work.
func ensureGitRepo(ctr *dagger.Container) *dagger.Container {
	return ctr.WithExec([]string{
		"sh", "-c",
		"if ! git rev-parse --git-dir >/dev/null 2>&1; then " +
			"rm -f .git && " +
			"git init -q && " +
			"git remote add origin https://github.com/macropower/kclipper.git && " +
			"git add -A && " +
			"git -c user.email=ci@dagger -c user.name=ci commit -q --allow-empty -m init; " +
			"fi",
	})
}
