// CI/CD functions for the kclipper project. Provides testing, linting,
// formatting, building, releasing, and development container support.

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/ci/internal/dagger"

	"golang.org/x/sync/errgroup"
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
	taskVersion         = "v3.45.0"        // renovate: datasource=github-releases depName=go-task/task
	conformVersion      = "v0.1.0-alpha.31" // renovate: datasource=github-releases depName=siderolabs/conform
	lefthookVersion     = "v2.1.1"         // renovate: datasource=github-releases depName=evilmartians/lefthook
	daggerVersion       = "v0.19.11"       // renovate: datasource=github-releases depName=dagger/dagger

	defaultRegistry = "ghcr.io/macropower/kclipper"

	// macosSDKFlags are the common compiler flags for macOS cross-compilation
	// via Zig, pointing to the vendored macOS SDK headers.
	macosSDKFlags = "-F/sdk/MacOSX.sdk/System/Library/Frameworks " +
		"-I/sdk/MacOSX.sdk/usr/include " +
		"-L/sdk/MacOSX.sdk/usr/lib " +
		"-Wno-availability -Wno-nullability-completeness"
)

// Ci provides CI/CD functions for kclipper. Create instances with [New].
type Ci struct {
	// Project source directory.
	Source *dagger.Directory
	// Container image registry address (e.g. "ghcr.io/macropower/kclipper").
	Registry string
}

// New creates a [Ci] module with the given project source directory.
func New(
	// Project source directory.
	// +defaultPath="/"
	// +ignore=["dist", ".worktrees", ".tmp", ".devcontainer"]
	source *dagger.Directory,
	// Container image registry address.
	// +optional
	registry string,
) *Ci {
	if registry == "" {
		registry = defaultRegistry
	}
	return &Ci{Source: source, Registry: registry}
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
	_, err := prettierBase().
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
// changeset against the original source directory.
//
// +generate
func (m *Ci) Format() *dagger.Changeset {
	// Go formatting via golangci-lint --fix.
	goFmt := m.lintBase().
		WithExec([]string{"golangci-lint", "run", "--fix"}).
		Directory("/src")

	// Prettier formatting.
	formatted := prettierBase().
		WithMountedDirectory("/src", goFmt).
		WithWorkdir("/src").
		WithExec([]string{
			"prettier", "--config", "./.prettierrc.yaml", "-w",
			"*.yaml", "*.md", "*.json",
			"**/*.yaml", "**/*.md", "**/*.json",
		}).
		Directory("/src")

	return formatted.Changes(m.Source)
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

// VersionTags returns the image tags derived from a version tag string.
// For example, "v1.2.3" yields ["latest", "v1.2.3", "v1", "v1.2"].
func (m *Ci) VersionTags(
	// Version tag (e.g. "v1.2.3").
	tag string,
) []string {
	return versionTags(tag)
}

// FormatDigestChecksums converts [Ci.PublishImages] output references to the
// checksums format expected by actions/attest-build-provenance. Each reference
// has the form "registry/image:tag@sha256:hex"; this function emits
// "hex  registry/image:tag" lines, deduplicating by digest.
func (m *Ci) FormatDigestChecksums(
	// Image references from [Ci.PublishImages] (e.g. "registry/image:tag@sha256:hex").
	refs []string,
) string {
	return formatDigestChecksums(refs)
}

// DeduplicateDigests returns unique image references from a list, keeping
// only the first occurrence of each sha256 digest.
func (m *Ci) DeduplicateDigests(
	// Image references (e.g. "registry/image:tag@sha256:hex").
	refs []string,
) []string {
	return deduplicateDigests(refs)
}

// BuildImages builds multi-arch runtime container images from a GoReleaser
// dist directory. If no dist is provided, a snapshot build is run.
func (m *Ci) BuildImages(
	// Version label for OCI metadata.
	// +default="snapshot"
	version string,
	// Pre-built GoReleaser dist directory. If not provided, runs a snapshot build.
	// +optional
	dist *dagger.Directory,
) []*dagger.Container {
	if dist == nil {
		dist = m.Build()
	}
	return runtimeImages(dist, version)
}

// PublishImages builds multi-arch container images using Dagger's native
// Container API and publishes them to the registry.
//
// Stable releases are published with multiple tags: :latest, :vX.Y.Z, :vX,
// :vX.Y. Pre-release versions are published with only their exact tag.
//
// +cache="never"
func (m *Ci) PublishImages(
	ctx context.Context,
	// Image tags to publish (e.g. ["latest", "v1.2.3", "v1", "v1.2"]).
	tags []string,
	// Registry username for authentication.
	// +optional
	registryUsername string,
	// Registry password or token for authentication.
	// +optional
	registryPassword *dagger.Secret,
	// Cosign private key for signing published images.
	// +optional
	cosignKey *dagger.Secret,
	// Password for the cosign private key. Required when the key is encrypted.
	// +optional
	cosignPassword *dagger.Secret,
	// Pre-built GoReleaser dist directory. If not provided, runs a snapshot build.
	// +optional
	dist *dagger.Directory,
) (string, error) {
	// Use the first non-"latest" tag as the version label, or fall back to "snapshot".
	version := "snapshot"
	for _, t := range tags {
		if t != "latest" {
			version = t
			break
		}
	}

	variants := m.BuildImages(version, dist)
	digests, err := m.publishImages(ctx, variants, tags, registryUsername, registryPassword, cosignKey, cosignPassword)
	if err != nil {
		return "", err
	}

	// Deduplicate digests for the summary (tags may share a manifest).
	unique := deduplicateDigests(digests)
	return fmt.Sprintf("published %d tags (%d unique digests)\n%s", len(tags), len(unique), strings.Join(digests, "\n")), nil
}

// publishImages publishes pre-built container image variants to the registry
// and returns the list of published image digests.
//
// This is the internal implementation shared by [Ci.PublishImages] and
// [Ci.Release].
func (m *Ci) publishImages(
	ctx context.Context,
	variants []*dagger.Container,
	tags []string,
	registryUsername string,
	registryPassword *dagger.Secret,
	cosignKey *dagger.Secret,
	cosignPassword *dagger.Secret,
) ([]string, error) {
	// Publish multi-arch manifest for each tag concurrently.
	publisher := dag.Container()
	if registryPassword != nil {
		publisher = publisher.WithRegistryAuth(registryHost(m.Registry), registryUsername, registryPassword)
	}

	digests := make([]string, len(tags))
	g, gCtx := errgroup.WithContext(ctx)
	for i, t := range tags {
		ref := fmt.Sprintf("%s:%s", m.Registry, t)
		g.Go(func() error {
			digest, err := publisher.Publish(gCtx, ref, dagger.ContainerPublishOpts{
				PlatformVariants: variants,
			})
			if err != nil {
				return fmt.Errorf("publish %s: %w", ref, err)
			}
			digests[i] = digest
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Sign each published image with cosign (key-based signing).
	// Deduplicate first -- multiple tags often share one manifest digest.
	if cosignKey != nil {
		toSign := deduplicateDigests(digests)

		cosignCtr := dag.Container().
			From("gcr.io/projectsigstore/cosign:" + cosignVersion).
			WithSecretVariable("COSIGN_KEY", cosignKey)
		if registryPassword != nil {
			cosignCtr = cosignCtr.WithRegistryAuth(registryHost(m.Registry), registryUsername, registryPassword)
		}
		if cosignPassword != nil {
			cosignCtr = cosignCtr.WithSecretVariable("COSIGN_PASSWORD", cosignPassword)
		}

		g, gCtx := errgroup.WithContext(ctx)
		for _, digest := range toSign {
			g.Go(func() error {
				_, err := cosignCtr.
					WithExec([]string{"cosign", "sign", "--key", "env://COSIGN_KEY", digest, "--yes"}).
					Sync(gCtx)
				if err != nil {
					return fmt.Errorf("sign image %s: %w", digest, err)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	return digests, nil
}

// deduplicateDigests returns unique image references from a list of
// "registry/image:tag@sha256:hex" strings, keeping only the first
// occurrence of each digest.
func deduplicateDigests(refs []string) []string {
	seen := make(map[string]bool)
	var unique []string
	for _, ref := range refs {
		parts := strings.SplitN(ref, "@sha256:", 2)
		if len(parts) != 2 {
			continue
		}
		if !seen[parts[1]] {
			seen[parts[1]] = true
			unique = append(unique, ref)
		}
	}
	return unique
}

// Release runs GoReleaser for binaries/archives/signing, then builds and
// publishes container images using Dagger-native Container.Publish().
// GoReleaser's Docker support is skipped entirely to avoid Docker-in-Docker.
//
// Returns the dist/ directory containing checksums.txt and digests.txt
// for attestation in the calling workflow.
//
// +cache="never"
func (m *Ci) Release(
	ctx context.Context,
	// GitHub token for creating the release.
	githubToken *dagger.Secret,
	// Registry username for container image authentication.
	registryUsername string,
	// Registry password or token for container image authentication.
	registryPassword *dagger.Secret,
	// Version tag to release (e.g. "v1.2.3").
	tag string,
	// Cosign private key for signing published images.
	// +optional
	cosignKey *dagger.Secret,
	// Password for the cosign private key. Required when the key is encrypted.
	// +optional
	cosignPassword *dagger.Secret,
	// macOS code signing PKCS#12 certificate (base64-encoded).
	// +optional
	macosSignP12 *dagger.Secret,
	// Password for the macOS code signing certificate.
	// +optional
	macosSignPassword *dagger.Secret,
	// Apple App Store Connect API key for notarization.
	// +optional
	macosNotaryKey *dagger.Secret,
	// Apple App Store Connect API key ID.
	// +optional
	macosNotaryKeyId *dagger.Secret,
	// Apple App Store Connect API issuer ID.
	// +optional
	macosNotaryIssuerId *dagger.Secret,
) (*dagger.Directory, error) {
	ctr := m.releaserBase().
		WithSecretVariable("GITHUB_TOKEN", githubToken)

	// Conditionally add cosign secrets for GoReleaser binary signing.
	// When no key is provided, signing is skipped entirely since keyless
	// OIDC signing is unavailable inside the Dagger container.
	skipFlags := "docker"
	if cosignKey != nil {
		ctr = ctr.WithSecretVariable("COSIGN_KEY", cosignKey)
		if cosignPassword != nil {
			ctr = ctr.WithSecretVariable("COSIGN_PASSWORD", cosignPassword)
		}
	} else {
		skipFlags = "docker,sign"
	}

	// Conditionally add macOS signing secrets.
	if macosSignP12 != nil {
		ctr = ctr.
			WithSecretVariable("MACOS_SIGN_P12", macosSignP12).
			WithSecretVariable("MACOS_SIGN_PASSWORD", macosSignPassword).
			WithSecretVariable("MACOS_NOTARY_KEY", macosNotaryKey).
			WithSecretVariable("MACOS_NOTARY_KEY_ID", macosNotaryKeyId).
			WithSecretVariable("MACOS_NOTARY_ISSUER_ID", macosNotaryIssuerId)
	}

	// Run GoReleaser for binaries, archives, Homebrew, Nix (and signing
	// when cosignKey is provided). Docker is always skipped -- images are
	// published natively via Dagger below.
	dist := ctr.
		WithExec([]string{"goreleaser", "release", "--clean", "--skip=" + skipFlags}).
		Directory("/src/dist")

	// Derive image tags from the version tag (e.g. v1.2.3 -> latest, v1.2.3, v1, v1.2).
	tags := versionTags(tag)

	// Publish multi-arch container images via Dagger-native API.
	// Reuse the dist directory from the goreleaser run to avoid building twice.
	variants := runtimeImages(dist, tag)
	digests, err := m.publishImages(ctx, variants, tags, registryUsername, registryPassword, cosignKey, cosignPassword)
	if err != nil {
		return nil, fmt.Errorf("publish images: %w", err)
	}

	// Write digests in checksums format for attest-build-provenance.
	// Dagger's Publish returns "registry/image:tag@sha256:hex" but the
	// action's subject-checksums input expects "hex  name" per sha256sum.
	if len(digests) > 0 {
		dist = dist.WithNewFile("digests.txt", formatDigestChecksums(digests))
	}

	return dist, nil
}

// ---------------------------------------------------------------------------
// Development
// ---------------------------------------------------------------------------

// Dev returns an interactive development container with project tools
// pre-installed. Pass optional config directories to enable Claude Code
// and git inside the container.
//
// Config directories are mounted as read-only snapshots; any changes
// made inside the container are ephemeral and will not persist back to
// the host.
//
// Usage:
//
//	dagger call dev terminal
//	dagger call dev --claude-config=~/.claude --git-config=~/.config/git terminal
//
// +cache="never"
func (m *Ci) Dev(
	// Claude Code configuration directory (~/.claude).
	// +optional
	claudeConfig *dagger.Directory,
	// Claude Code settings file (~/.claude.json).
	// +optional
	claudeJSON *dagger.File,
	// Git configuration directory (~/.config/git).
	// +optional
	gitConfig *dagger.Directory,
) *dagger.Container {
	ctr := ensureGitRepo(dag.Container().
		From("golang:"+goVersion).
		WithExec([]string{"sh", "-c",
			"apt-get update && apt-get install -y --no-install-recommends " +
				"curl less man-db gnupg2 nano vim xz-utils jq wget " +
				"&& apt-get clean && rm -rf /var/lib/apt/lists/*",
		}).
		WithExec([]string{"go", "install", "github.com/go-task/task/v3/cmd/task@" + taskVersion}).
		WithExec([]string{"go", "install", "github.com/siderolabs/conform/cmd/conform@" + conformVersion}).
		WithExec([]string{"go", "install", "github.com/evilmartians/lefthook/v2@" + lefthookVersion}).
		WithExec([]string{"sh", "-c", "curl -fsSL https://dl.dagger.io/dagger/install.sh | DAGGER_VERSION=" + daggerVersion + " BIN_DIR=/usr/local/bin sh"}).
		WithExec([]string{"sh", "-c", "wget -O - https://claude.ai/install.sh | bash"}).
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		// Pre-download Go modules so the first build/test is fast.
		WithExec([]string{"go", "mod", "download"}).
		WithEnvVariable("EDITOR", "nano").
		WithEnvVariable("VISUAL", "nano").
		WithEnvVariable("TERM", "xterm-256color").
		WithEnvVariable("PATH", "/root/.local/bin:$PATH",
			dagger.ContainerWithEnvVariableOpts{Expand: true}))

	if claudeConfig != nil {
		ctr = ctr.WithMountedDirectory("/root/.claude", claudeConfig)
	}
	if claudeJSON != nil {
		ctr = ctr.WithMountedFile("/root/.claude.json", claudeJSON)
	}
	if gitConfig != nil {
		ctr = ctr.WithMountedDirectory("/root/.config/git", gitConfig)
	}

	// ExperimentalPrivilegedNesting gives the terminal session a
	// connection to the parent Dagger engine, so nested `dagger call`
	// invocations work without mounting the Docker socket.
	return ctr.WithDefaultTerminalCmd([]string{"bash"},
		dagger.ContainerWithDefaultTerminalCmdOpts{
			ExperimentalPrivilegedNesting: true,
		})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// registryHost extracts the host (with optional port) from a registry address.
// For example, "ghcr.io/macropower/kclipper" returns "ghcr.io".
func registryHost(registry string) string {
	return strings.SplitN(registry, "/", 2)[0]
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
		// GoReleaser uses the build id (kclipper), not the binary name (kcl).
		// Directory names include the GOAMD64/GOARM64 version suffix:
		//   amd64 -> kclipper_linux_amd64_v1
		//   arm64 -> kclipper_linux_arm64_v8.0
		dir := "kclipper_linux_amd64_v1"
		if platform == "linux/arm64" {
			dir = "kclipper_linux_arm64_v8.0"
		}

		ctr := dag.Container(dagger.ContainerOpts{Platform: platform}).
			From("debian:13-slim").
			// OCI labels (container config) for metadata.
			WithLabel("org.opencontainers.image.title", "kclipper").
			WithLabel("org.opencontainers.image.description", "A superset of KCL that integrates Helm chart management").
			WithLabel("org.opencontainers.image.source", "https://github.com/macropower/kclipper").
			WithLabel("org.opencontainers.image.url", "https://github.com/macropower/kclipper").
			WithLabel("org.opencontainers.image.version", version).
			WithLabel("org.opencontainers.image.created", created).
			WithLabel("org.opencontainers.image.licenses", "Apache-2.0").
			// OCI annotations (manifest-level) for registry discoverability.
			WithAnnotation("org.opencontainers.image.title", "kclipper").
			WithAnnotation("org.opencontainers.image.version", version).
			WithAnnotation("org.opencontainers.image.created", created).
			WithAnnotation("org.opencontainers.image.source", "https://github.com/macropower/kclipper").
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
			WithFile("/usr/local/bin/kcl", dist.File(dir+"/kcl")).
			WithEntrypoint([]string{"kcl"})

		variants = append(variants, ctr)
	}

	return variants
}

// formatDigestChecksums converts Dagger Publish output references to the
// checksums format expected by actions/attest-build-provenance's
// subject-checksums input. Each reference has the form
// "registry/image:tag@sha256:hex"; this function emits "hex  registry/image:tag"
// lines, deduplicating by digest.
func formatDigestChecksums(refs []string) string {
	seen := make(map[string]bool)
	var b strings.Builder
	for _, ref := range refs {
		parts := strings.SplitN(ref, "@sha256:", 2)
		if len(parts) != 2 {
			continue
		}
		hex := parts[1]
		if seen[hex] {
			continue
		}
		seen[hex] = true
		fmt.Fprintf(&b, "%s  %s\n", hex, parts[0])
	}
	return b.String()
}

// versionTags derives the set of image tags from a version tag string.
// Stable releases get "latest" and shorthand tags (e.g. "v1.2.3" yields
// ["latest", "v1.2.3", "v1", "v1.2"]). Pre-release versions only get their
// exact tag (e.g. "v1.0.0-rc.1" yields ["v1.0.0-rc.1"]).
func versionTags(tag string) []string {
	v := strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(v, ".", 3)

	// Detect pre-release: any version component contains a hyphen
	// (e.g. "1.0.0-rc.1" → third part is "0-rc.1").
	for _, p := range parts {
		if strings.Contains(p, "-") {
			return []string{tag}
		}
	}

	tags := []string{"latest", tag}
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

// prettierBase returns a Node container with prettier pre-installed.
// Callers must mount their source directory and set the workdir.
func prettierBase() *dagger.Container {
	return dag.Container().
		From("node:lts-slim").
		WithExec([]string{"npm", "install", "-g", "prettier@" + prettierVersion})
}

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
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache")
}

// goToolchain returns a configured [dagger.Go] toolchain for the project source.
func (m *Ci) goToolchain() *dagger.Go {
	return dag.Go(dagger.GoOpts{
		Source:      m.Source,
		Version:     goVersion,
		ModuleCache: dag.CacheVolume("go-mod"),
		BuildCache:  dag.CacheVolume("go-build"),
		Cgo:         true,
	})
}

// goBase returns a Go container with source, module cache, and build cache.
func (m *Ci) goBase() *dagger.Container {
	return ensureGitRepo(m.goToolchain().Env())
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
		WithEnvVariable("CC_DARWIN_AMD64", "/src/hack/zig-macos-wrapper.sh -target x86_64-macos-none "+macosSDKFlags).
		WithEnvVariable("CC_DARWIN_ARM64", "/src/hack/zig-macos-wrapper.sh -target aarch64-macos-none "+macosSDKFlags).
		WithEnvVariable("CXX_LINUX_AMD64", "/src/hack/zig-gold-wrapper.sh -target x86_64-linux-gnu").
		WithEnvVariable("CXX_LINUX_ARM64", "/src/hack/zig-gold-wrapper.sh -target aarch64-linux-gnu").
		WithEnvVariable("CXX_DARWIN_AMD64", "/src/hack/zig-macos-wrapper.sh -target x86_64-macos-none "+macosSDKFlags).
		WithEnvVariable("CXX_DARWIN_ARM64", "/src/hack/zig-macos-wrapper.sh -target aarch64-macos-none "+macosSDKFlags))
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
