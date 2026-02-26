// CI/CD functions for the kclipper project. Provides testing, linting,
// formatting, building, releasing, and development container support.
// Generic CI functions are delegated to the [Go] toolchain module; this module
// adds kclipper-specific build, release, and runtime image logic.

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/kclipper-dev/internal/dagger"

	"golang.org/x/sync/errgroup"
)

const (
	goVersion   = "1.25"            // renovate: datasource=golang-version depName=go
	zigVersion  = "0.15.2"          // renovate: datasource=github-releases depName=ziglang/zig
	cosignVersion = "v3.0.4"       // renovate: datasource=github-releases depName=sigstore/cosign
	syftVersion = "v1.41.1"         // renovate: datasource=github-releases depName=anchore/syft
	kclLSPVersion = "v0.11.2"      // renovate: datasource=github-releases depName=kcl-lang/kcl
	goreleaserVersion = "v2.13.3"  // renovate: datasource=github-releases depName=goreleaser/goreleaser

	defaultRegistry = "ghcr.io/macropower/kclipper"

	kclipperCloneURL = "https://github.com/macropower/kclipper.git"

	// macosSDKFlags are the common compiler flags for macOS cross-compilation
	// via Zig, pointing to the vendored macOS SDK headers.
	macosSDKFlags = "-F/sdk/MacOSX.sdk/System/Library/Frameworks " +
		"-I/sdk/MacOSX.sdk/usr/include " +
		"-L/sdk/MacOSX.sdk/usr/lib " +
		"-Wno-availability -Wno-nullability-completeness"
)

// KclipperDev provides CI/CD functions for kclipper. Create instances with [New].
type KclipperDev struct {
	// Project source directory.
	Source *dagger.Directory
	// Directory containing only go.mod and go.sum, synced independently of
	// [KclipperDev.Source] so that its content hash changes only when dependency
	// files change. Used by [Go.GoModBase] to cache go mod download.
	GoMod *dagger.Directory
	// Container image registry address (e.g. "ghcr.io/macropower/kclipper").
	Registry string
}

// New creates a [KclipperDev] module with the given project source directory.
func New(
	// Project source directory.
	// +defaultPath="/"
	// +ignore=["dist", ".worktrees", ".tmp", ".git"]
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
) *KclipperDev {
	if registry == "" {
		registry = defaultRegistry
	}
	return &KclipperDev{Source: source, GoMod: goMod, Registry: registry}
}

// goToolchain returns the configured [Go] toolchain module instance for delegation.
func (m *KclipperDev) goToolchain() *dagger.Go {
	return dag.Go(dagger.GoOpts{
		Source:   m.Source,
		GoMod:    m.GoMod,
		Registry: m.Registry,
	})
}

// devToolchain returns the configured [Dev] toolchain module instance for delegation.
func (m *KclipperDev) devToolchain() *dagger.Dev {
	return dag.Dev(dagger.DevOpts{Source: m.Source})
}

// ---------------------------------------------------------------------------
// Testing (delegated to go toolchain)
// ---------------------------------------------------------------------------

// Test runs the Go test suite. Uses only cacheable flags so that Go's
// internal test result cache (GOCACHE) can skip unchanged packages
// across runs via the persistent go-build cache volume.
//
// +check
func (m *KclipperDev) Test(ctx context.Context) error {
	return m.goToolchain().Test(ctx)
}

// TestCoverage runs Go tests with coverage profiling and returns the
// profile file. Runs independently of [KclipperDev.Test] because -coverprofile
// disables Go's internal test result caching. Dagger's layer caching
// still shares the base container layers (image, module download) with
// [KclipperDev.Test].
func (m *KclipperDev) TestCoverage() *dagger.File {
	return m.goToolchain().TestCoverage()
}

// ---------------------------------------------------------------------------
// Linting (delegated to go toolchain)
// ---------------------------------------------------------------------------

// Lint runs golangci-lint on the source code.
//
// +check
func (m *KclipperDev) Lint(ctx context.Context) error {
	return m.goToolchain().Lint(ctx)
}

// LintPrettier checks YAML, JSON, and Markdown formatting.
//
// +check
func (m *KclipperDev) LintPrettier(ctx context.Context) error {
	return m.goToolchain().LintPrettier(ctx)
}

// LintActions runs zizmor to lint GitHub Actions workflows.
//
// +check
func (m *KclipperDev) LintActions(ctx context.Context) error {
	return m.goToolchain().LintActions(ctx)
}

// LintReleaser validates the GoReleaser configuration. Uses
// [Go.GoreleaserCheckBase] with the kclipper remote URL because the
// goreleaser config references a git remote for homebrew/nix repository
// resolution.
//
// +check
func (m *KclipperDev) LintReleaser(ctx context.Context) error {
	_, err := m.goToolchain().GoreleaserCheckBase(dagger.GoGoreleaserCheckBaseOpts{
		RemoteURL: kclipperCloneURL,
	}).
		WithExec([]string{"goreleaser", "check"}).
		Sync(ctx)
	return err
}

// LintDeadcode reports unreachable functions in the codebase using the
// golang.org/x/tools deadcode analyzer. This is an advisory lint that
// is not included in standard checks; invoke via dagger call lint-deadcode.
func (m *KclipperDev) LintDeadcode(ctx context.Context) error {
	return m.goToolchain().LintDeadcode(ctx)
}

// LintCommitMsg validates a commit message against the project's conventional
// commit policy using conform. The message file is typically provided by a
// git commit-msg hook.
func (m *KclipperDev) LintCommitMsg(
	ctx context.Context,
	// Commit message file to validate (e.g. .git/COMMIT_EDITMSG).
	msgFile *dagger.File,
) error {
	return m.goToolchain().LintCommitMsg(ctx, msgFile)
}

// ---------------------------------------------------------------------------
// Benchmarking (delegated to go toolchain + kclipper build stage)
// ---------------------------------------------------------------------------

// BenchmarkResult holds the timing for a single pipeline stage.
type BenchmarkResult struct {
	// Pipeline stage name (e.g. "goBase", "lint", "test").
	Name string
	// Duration in seconds.
	DurationSecs float64
	// Whether the stage completed successfully.
	Ok bool
	// Error message if the stage failed.
	Error string
}

// Benchmark measures the wall-clock time of key pipeline stages and
// returns structured results. Use this to identify bottlenecks and track
// performance regressions. Each stage is run sequentially to isolate
// timings.
//
// +cache="never"
func (m *KclipperDev) Benchmark(ctx context.Context) ([]*BenchmarkResult, error) {
	return m.runBenchmarks(ctx, false)
}

// BenchmarkSummary measures the wall-clock time of key pipeline stages
// and returns a human-readable table. This is a convenience wrapper
// around [KclipperDev.Benchmark] for CLI use without jq post-processing.
//
// When parallel is true, all stages run concurrently to measure the
// real-world wall-clock time of the full CI pipeline. The total row
// shows overall elapsed time rather than the sum of individual stages.
//
// +cache="never"
func (m *KclipperDev) BenchmarkSummary(
	ctx context.Context,
	// Run stages concurrently to measure full-pipeline wall-clock time.
	// +default=false
	parallel bool,
) (string, error) {
	results, err := m.runBenchmarks(ctx, parallel)
	if err != nil {
		return "", err
	}
	return formatBenchmarkTable(results, parallel), nil
}

// formatBenchmarkTable formats benchmark results as an aligned text table.
func formatBenchmarkTable(results []*BenchmarkResult, parallel bool) string {
	var b strings.Builder

	mode := "sequential"
	if parallel {
		mode = "parallel"
	}
	fmt.Fprintf(&b, "Benchmark (%s)\n", mode)
	fmt.Fprintf(&b, "%-20s %10s %8s\n", "STAGE", "DURATION", "STATUS")
	fmt.Fprintf(&b, "%-20s %10s %8s\n", "-----", "--------", "------")

	var total float64
	var maxDur float64
	allOk := true
	for _, r := range results {
		status := "ok"
		if !r.Ok {
			status = "FAIL"
			allOk = false
		}
		fmt.Fprintf(&b, "%-20s %9.1fs %8s\n", r.Name, r.DurationSecs, status)
		total += r.DurationSecs
		if r.DurationSecs > maxDur {
			maxDur = r.DurationSecs
		}
	}

	fmt.Fprintf(&b, "%-20s %10s %8s\n", "-----", "--------", "------")

	// In parallel mode, show both the wall-clock (max) and sum of stages.
	totalStatus := "ok"
	if !allOk {
		totalStatus = "FAIL"
	}
	if parallel {
		fmt.Fprintf(&b, "%-20s %9.1fs %8s\n", "WALL-CLOCK", maxDur, totalStatus)
		fmt.Fprintf(&b, "%-20s %9.1fs\n", "SUM", total)
	} else {
		fmt.Fprintf(&b, "%-20s %9.1fs %8s\n", "TOTAL", total, totalStatus)
	}

	return b.String()
}

// benchmarkStage pairs a stage name with its execution function.
type benchmarkStage struct {
	name string
	fn   func(context.Context) error
}

// benchmarkStages returns the list of pipeline stages to benchmark,
// including the kclipper-specific build stage.
func (m *KclipperDev) benchmarkStages() []benchmarkStage {
	gt := m.goToolchain()
	return []benchmarkStage{
		{"goBase", func(ctx context.Context) error {
			_, err := gt.CacheBust(gt.GoBase()).Sync(ctx)
			return err
		}},
		{"lint", func(ctx context.Context) error {
			_, err := gt.CacheBust(gt.LintBase()).
				WithExec([]string{"golangci-lint", "run"}).
				Sync(ctx)
			return err
		}},
		{"test", func(ctx context.Context) error {
			_, err := gt.CacheBust(gt.GoBase()).
				WithExec([]string{"go", "test", "./..."}).
				Sync(ctx)
			return err
		}},
		{"lint-prettier", func(ctx context.Context) error {
			_, err := gt.CacheBust(gt.PrettierBase()).
				WithMountedDirectory("/src", m.Source).
				WithWorkdir("/src").
				WithExec([]string{
					"prettier", "--config", "./.prettierrc.yaml", "--check",
					"*.yaml", "*.md", "*.json",
					"**/*.yaml", "**/*.md", "**/*.json",
				}).
				Sync(ctx)
			return err
		}},
		{"lint-actions", func(ctx context.Context) error {
			_, err := gt.CacheBust(dag.Container().
				From("ghcr.io/zizmorcore/zizmor:1.22.0")).
				WithMountedDirectory("/src", m.Source).
				WithWorkdir("/src").
				WithExec([]string{
					"zizmor", ".github/workflows", "--config", ".github/zizmor.yaml",
				}).
				Sync(ctx)
			return err
		}},
		{"lint-releaser", func(ctx context.Context) error {
			_, err := gt.CacheBust(gt.GoreleaserCheckBase(dagger.GoGoreleaserCheckBaseOpts{
				RemoteURL: kclipperCloneURL,
			})).
				WithExec([]string{"goreleaser", "check"}).
				Sync(ctx)
			return err
		}},
		{"build", func(ctx context.Context) error {
			_, err := gt.CacheBust(m.releaserBase()).
				WithExec([]string{
					"goreleaser", "release", "--snapshot", "--clean",
					"--skip=docker,homebrew,nix,sign,sbom",
					"--parallelism=0",
				}).
				Sync(ctx)
			return err
		}},
	}
}

// runBenchmarks executes benchmark stages. When parallel is false, stages
// run sequentially for isolated timings. When true, stages run concurrently
// to measure real-world wall-clock time.
func (m *KclipperDev) runBenchmarks(ctx context.Context, parallel bool) ([]*BenchmarkResult, error) {
	stages := m.benchmarkStages()

	if parallel {
		return m.runBenchmarksParallel(ctx, stages)
	}
	return m.runBenchmarksSequential(ctx, stages)
}

// runBenchmarksSequential runs each stage one at a time for isolated timings.
func (m *KclipperDev) runBenchmarksSequential(ctx context.Context, stages []benchmarkStage) ([]*BenchmarkResult, error) {
	results := make([]*BenchmarkResult, 0, len(stages))
	for _, s := range stages {
		start := time.Now()
		err := s.fn(ctx)
		elapsed := time.Since(start).Seconds()

		r := &BenchmarkResult{
			Name:         s.name,
			DurationSecs: elapsed,
			Ok:           err == nil,
		}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	return results, nil
}

// runBenchmarksParallel runs all stages concurrently and reports individual
// wall-clock times. This measures what a real CI run looks like when Dagger
// evaluates pipelines in parallel.
func (m *KclipperDev) runBenchmarksParallel(ctx context.Context, stages []benchmarkStage) ([]*BenchmarkResult, error) {
	results := make([]*BenchmarkResult, len(stages))
	g, gCtx := errgroup.WithContext(ctx)

	for i, s := range stages {
		g.Go(func() error {
			start := time.Now()
			err := s.fn(gCtx)
			elapsed := time.Since(start).Seconds()

			r := &BenchmarkResult{
				Name:         s.name,
				DurationSecs: elapsed,
				Ok:           err == nil,
			}
			if err != nil {
				r.Error = err.Error()
			}
			results[i] = r
			return nil // always collect results, don't abort on stage failure
		})
	}

	_ = g.Wait()
	return results, nil
}

// ---------------------------------------------------------------------------
// Formatting (delegated to go toolchain)
// ---------------------------------------------------------------------------

// Format runs golangci-lint --fix and prettier --write, returning the
// changeset against the original source directory.
//
// +generate
func (m *KclipperDev) Format() *dagger.Changeset {
	return m.goToolchain().Format()
}

// ---------------------------------------------------------------------------
// Generation (delegated to go toolchain)
// ---------------------------------------------------------------------------

// Generate runs go generate and returns the changeset of generated files
// against the original source. The project's gen.go directive produces KCL
// module files under modules/helm/.
//
// +generate
func (m *KclipperDev) Generate() *dagger.Changeset {
	return m.goToolchain().Generate()
}

// ---------------------------------------------------------------------------
// Building (kclipper-specific)
// ---------------------------------------------------------------------------

// Build runs GoReleaser in snapshot mode, producing binaries for all
// platforms. Returns the dist/ directory. Source archives are skipped in
// snapshot mode since they are only needed for releases.
func (m *KclipperDev) Build() *dagger.Directory {
	return m.releaserBase().
		WithExec([]string{
			"goreleaser", "release", "--snapshot", "--clean",
			"--skip=docker,homebrew,nix,sign,sbom",
			"--parallelism=0",
		}).
		Directory("/src/dist")
}

// VersionTags returns the image tags derived from a version tag string.
// For example, "v1.2.3" yields ["latest", "v1.2.3", "v1", "v1.2"].
func (m *KclipperDev) VersionTags(
	ctx context.Context,
	// Version tag (e.g. "v1.2.3").
	tag string,
) ([]string, error) {
	return m.goToolchain().VersionTags(ctx, tag)
}

// FormatDigestChecksums converts [KclipperDev.PublishImages] output references to the
// checksums format expected by actions/attest-build-provenance. Each reference
// has the form "registry/image:tag@sha256:hex"; this function emits
// "hex  registry/image:tag" lines, deduplicating by digest.
func (m *KclipperDev) FormatDigestChecksums(
	ctx context.Context,
	// Image references from [KclipperDev.PublishImages] (e.g. "registry/image:tag@sha256:hex").
	refs []string,
) (string, error) {
	return m.goToolchain().FormatDigestChecksums(ctx, refs)
}

// DeduplicateDigests returns unique image references from a list, keeping
// only the first occurrence of each sha256 digest.
func (m *KclipperDev) DeduplicateDigests(
	ctx context.Context,
	// Image references (e.g. "registry/image:tag@sha256:hex").
	refs []string,
) ([]string, error) {
	return m.goToolchain().DeduplicateDigests(ctx, refs)
}

// RegistryHost extracts the host (with optional port) from a registry
// address. For example, "ghcr.io/macropower/kclipper" returns "ghcr.io".
func (m *KclipperDev) RegistryHost(
	ctx context.Context,
	// Registry address (e.g. "ghcr.io/macropower/kclipper").
	registry string,
) (string, error) {
	return m.goToolchain().RegistryHost(ctx, registry)
}

// BuildImages builds multi-arch runtime container images from a GoReleaser
// dist directory. If no dist is provided, a snapshot build is run.
func (m *KclipperDev) BuildImages(
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
func (m *KclipperDev) PublishImages(
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
	digests, err := m.goToolchain().PublishImages(ctx, variants, tags, dagger.GoPublishImagesOpts{
		RegistryUsername:  registryUsername,
		RegistryPassword: registryPassword,
		CosignKey:        cosignKey,
		CosignPassword:   cosignPassword,
	})
	if err != nil {
		return "", err
	}

	// Deduplicate digests for the summary (tags may share a manifest).
	unique, err := m.DeduplicateDigests(ctx, digests)
	if err != nil {
		return "", fmt.Errorf("deduplicate digests: %w", err)
	}
	return fmt.Sprintf("published %d tags (%d unique digests)\n%s", len(tags), len(unique), strings.Join(digests, "\n")), nil
}

// Release runs GoReleaser for binaries/archives/signing, then builds and
// publishes container images using Dagger-native Container.Publish().
// GoReleaser's Docker support is skipped entirely to avoid Docker-in-Docker.
//
// Returns the dist/ directory containing checksums.txt and digests.txt
// for attestation in the calling workflow.
//
// +cache="never"
func (m *KclipperDev) Release(
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

	// Derive image tags from the version tag.
	tags, err := m.VersionTags(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("version tags: %w", err)
	}

	// Publish multi-arch container images via Dagger-native API.
	variants := runtimeImages(dist, tag)
	digests, err := m.goToolchain().PublishImages(ctx, variants, tags, dagger.GoPublishImagesOpts{
		RegistryUsername:  registryUsername,
		RegistryPassword: registryPassword,
		CosignKey:        cosignKey,
		CosignPassword:   cosignPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("publish images: %w", err)
	}

	// Write digests in checksums format for attest-build-provenance.
	if len(digests) > 0 {
		checksums, err := m.FormatDigestChecksums(ctx, digests)
		if err != nil {
			return nil, fmt.Errorf("format digest checksums: %w", err)
		}
		dist = dist.WithNewFile("digests.txt", checksums)
	}

	return dist, nil
}

// ---------------------------------------------------------------------------
// Development (delegated to go toolchain with kclipper clone URL)
// ---------------------------------------------------------------------------

// DevBase returns the base development container with all tools
// pre-installed but no source mounted. Used by integration tests to
// verify tool availability without requiring an interactive terminal.
func (m *KclipperDev) DevBase() *dagger.Container {
	return m.devToolchain().DevBase()
}

// DevEnv returns a development container with the git repository cloned,
// the requested branch checked out, and local source files overlaid.
// Cache volumes provide per-branch workspace isolation and shared Go
// module/build caches. Unlike [KclipperDev.Dev], this does not open an interactive
// terminal or export results.
func (m *KclipperDev) DevEnv(
	// Branch to check out in the dev container. Each branch gets its
	// own Dagger cache volume for workspace isolation.
	branch string,
	// Base branch name used when creating a new branch that does not
	// exist locally or on the remote. Looked up as origin/<base> in
	// the container clone. Defaults to "main" when empty.
	// +optional
	base string,
) *dagger.Container {
	return m.devToolchain().DevEnv(branch, kclipperCloneURL, dagger.DevDevEnvOpts{
		Base: base,
	})
}

// Dev opens an interactive development container with a real git
// repository and returns the modified source directory when the session
// ends. The container is created via [KclipperDev.DevEnv], which clones the
// upstream repo (blobless) and checks out the specified branch, enabling
// pushes, rebases, and other git operations.
//
// Source files from the project directory are overlaid on top of the
// checked-out branch, bringing in local uncommitted changes. Each branch
// gets its own Dagger cache volume for workspace isolation.
//
// The returned directory includes .git with full commit history. Use the
// Taskfile dev/claude tasks to handle the translation between the
// container's standalone .git and the host's worktree format.
//
// Usage:
//
//	task dev                        # defaults to current branch
//	task dev BRANCH=feat/my-work    # explicit branch, base = current branch
//	task claude BRANCH=feat/my-work BASE=main  # explicit base
//
// +cache="never"
func (m *KclipperDev) Dev(
	// Branch to check out in the dev container. Each branch gets its
	// own Dagger cache volume for workspace isolation.
	branch string,
	// Base branch name used when creating a new branch that does not
	// exist locally or on the remote. Looked up as origin/<base> in
	// the container clone. Defaults to "main" when empty.
	// +optional
	base string,
	// Claude Code configuration directory (~/.claude).
	// +optional
	// +ignore=["debug", "projects", "todos", "file-history", "plans", "tasks", "teams", "session-env", "backups", "paste-cache", "cache", "telemetry", "downloads", "shell-snapshots", "history.jsonl", ".claude.json*", "stats-cache.json", "statsig", "skills"]
	claudeConfig *dagger.Directory,
	// Claude Code settings file (~/.claude.json).
	// +optional
	claudeJSON *dagger.File,
	// Git configuration directory (~/.config/git).
	// +optional
	gitConfig *dagger.Directory,
	// Claude Code status line configuration directory (~/.config/ccstatusline).
	// +optional
	ccstatuslineConfig *dagger.Directory,
	// Timezone for the container (e.g. "America/New_York").
	// +optional
	tz string,
	// COLORTERM value (e.g. "truecolor").
	// +optional
	colorterm string,
	// TERM_PROGRAM value (e.g. "Apple_Terminal", "iTerm.app").
	// +optional
	termProgram string,
	// TERM_PROGRAM_VERSION value.
	// +optional
	termProgramVersion string,
	// Command to run in the terminal session. Defaults to ["zsh"].
	// +optional
	cmd []string,
) *dagger.Directory {
	return m.devToolchain().Dev(branch, kclipperCloneURL, dagger.DevDevOpts{
		Base:               base,
		ClaudeConfig:       claudeConfig,
		ClaudeJSON:         claudeJSON,
		GitConfig:          gitConfig,
		CcstatuslineConfig: ccstatuslineConfig,
		Tz:                 tz,
		Colorterm:          colorterm,
		TermProgram:        termProgram,
		TermProgramVersion: termProgramVersion,
		Cmd:                cmd,
	})
}

// ---------------------------------------------------------------------------
// Helpers (kclipper-specific)
// ---------------------------------------------------------------------------

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

// releaserBase returns a container with Go, GoReleaser, Zig, cosign, syft,
// pre-downloaded KCL Language Server binaries, and macOS SDK headers needed
// for CGO cross-compilation.
func (m *KclipperDev) releaserBase() *dagger.Container {
	ctr := dag.Container().
		From("golang:"+goVersion).
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
		// Install GoReleaser from its official OCI image.
		WithFile("/usr/local/bin/goreleaser",
			dag.Container().From("ghcr.io/goreleaser/goreleaser:"+goreleaserVersion).
				File("/usr/bin/goreleaser")).
		// Install cosign from its official OCI image.
		WithFile("/usr/local/bin/cosign",
			dag.Container().From("gcr.io/projectsigstore/cosign:"+cosignVersion).
				File("/ko-app/cosign")).
		// Install syft from its official OCI image.
		WithFile("/usr/local/bin/syft",
			dag.Container().From("ghcr.io/anchore/syft:"+syftVersion).
				File("/syft")).
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
		})

	// Pre-download Go modules using the GoMod-only directory.
	ctr = m.goToolchain().GoModBase(ctr, dagger.GoGoModBaseOpts{Src: m.Source})

	return m.goToolchain().EnsureGitRepo(ctr, dagger.GoEnsureGitRepoOpts{
		RemoteURL: kclipperCloneURL,
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
		WithEnvVariable("CXX_DARWIN_ARM64", "/src/hack/zig-macos-wrapper.sh -target aarch64-macos-none "+macosSDKFlags)
}
