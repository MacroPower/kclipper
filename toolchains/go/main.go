// Reusable Go CI functions for testing, linting, formatting, and
// publishing. Provides common pipeline stages that any Go project can
// consume.

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/go/internal/dagger"

	"golang.org/x/sync/errgroup"
)

const (
	goVersion           = "1.25"            // renovate: datasource=golang-version depName=go
	golangciLintVersion = "v2.9"            // renovate: datasource=github-releases depName=golangci/golangci-lint
	goreleaserVersion   = "v2.13.3"         // renovate: datasource=github-releases depName=goreleaser/goreleaser
	prettierVersion     = "3.5.3"           // renovate: datasource=npm depName=prettier
	zizmorVersion       = "1.22.0"          // renovate: datasource=github-releases depName=zizmorcore/zizmor
	deadcodeVersion     = "v0.42.0"         // renovate: datasource=go depName=golang.org/x/tools
	conformVersion      = "v0.1.0-alpha.31" // renovate: datasource=github-releases depName=siderolabs/conform
	cosignVersion       = "v3.0.4"          // renovate: datasource=github-releases depName=sigstore/cosign
)

// Go provides reusable Go CI functions for testing, linting, formatting,
// and publishing. Create instances with [New].
type Go struct {
	// Project source directory.
	Source *dagger.Directory
	// Directory containing only go.mod and go.sum, synced independently of
	// [Go.Source] so that its content hash changes only when dependency
	// files change. Used by [Go.GoModBase] to cache go mod download.
	GoMod *dagger.Directory
	// Container image registry address (e.g. "ghcr.io/org/image").
	Registry string
}

// New creates a [Go] module with the given project source directory.
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
) *Go {
	return &Go{Source: source, GoMod: goMod, Registry: registry}
}

// ---------------------------------------------------------------------------
// Testing
// ---------------------------------------------------------------------------

// Test runs the Go test suite. Uses only cacheable flags so that Go's
// internal test result cache (GOCACHE) can skip unchanged packages
// across runs via the persistent go-build cache volume.
func (m *Go) Test(ctx context.Context) error {
	_, err := m.GoBase().
		WithExec([]string{"go", "test", "./..."}).
		Sync(ctx)
	return err
}

// TestCoverage runs Go tests with coverage profiling and returns the
// profile file. Runs independently of [Go.Test] because -coverprofile
// disables Go's internal test result caching. Dagger's layer caching
// still shares the base container layers (image, module download) with
// [Go.Test].
func (m *Go) TestCoverage() *dagger.File {
	return m.GoBase().
		WithExec([]string{
			"go", "test", "-race", "-coverprofile=/tmp/coverage.txt", "./...",
		}).
		File("/tmp/coverage.txt")
}

// ---------------------------------------------------------------------------
// Linting
// ---------------------------------------------------------------------------

// Lint runs golangci-lint on the source code.
func (m *Go) Lint(ctx context.Context) error {
	_, err := m.LintBase().
		WithExec([]string{"golangci-lint", "run"}).
		Sync(ctx)
	return err
}

// LintPrettier checks YAML, JSON, and Markdown formatting.
func (m *Go) LintPrettier(
	ctx context.Context,
	// Prettier config file path relative to source root.
	// +optional
	configPath string,
	// File patterns to check.
	// +optional
	patterns []string,
) error {
	if configPath == "" {
		configPath = "./.prettierrc.yaml"
	}
	if len(patterns) == 0 {
		patterns = defaultPrettierPatterns()
	}
	args := append([]string{"prettier", "--config", configPath, "--check"}, patterns...)
	_, err := m.PrettierBase().
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec(args).
		Sync(ctx)
	return err
}

// LintActions runs zizmor to lint GitHub Actions workflows.
func (m *Go) LintActions(ctx context.Context) error {
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

// LintReleaser validates the GoReleaser configuration. Uses
// [Go.GoreleaserCheckBase] instead of a full release environment since
// goreleaser check only validates config syntax.
func (m *Go) LintReleaser(ctx context.Context) error {
	_, err := m.GoreleaserCheckBase("").
		WithExec([]string{"goreleaser", "check"}).
		Sync(ctx)
	return err
}

// LintDeadcode reports unreachable functions in the codebase using the
// golang.org/x/tools deadcode analyzer. This is an advisory lint that
// is not included in standard checks; invoke via dagger call lint-deadcode.
func (m *Go) LintDeadcode(ctx context.Context) error {
	_, err := m.GoBase().
		WithExec([]string{
			"go", "install",
			"golang.org/x/tools/cmd/deadcode@" + deadcodeVersion,
		}).
		WithExec([]string{"deadcode", "./..."}).
		Sync(ctx)
	return err
}

// LintCommitMsg validates a commit message against the project's conventional
// commit policy using conform. The message file is typically provided by a
// git commit-msg hook.
func (m *Go) LintCommitMsg(
	ctx context.Context,
	// Commit message file to validate (e.g. .git/COMMIT_EDITMSG).
	msgFile *dagger.File,
) error {
	ctr := dag.Container().
		From("alpine/git:latest").
		WithFile("/usr/local/bin/conform",
			dag.Container().From("ghcr.io/siderolabs/conform:"+conformVersion).
				File("/conform")).
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src")

	_, err := m.EnsureGitInit(ctr).
		WithMountedFile("/tmp/commit-msg", msgFile).
		WithExec([]string{"conform", "enforce", "--commit-msg-file", "/tmp/commit-msg"}).
		Sync(ctx)
	return err
}

// ---------------------------------------------------------------------------
// Benchmarking
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
func (m *Go) Benchmark(ctx context.Context) ([]*BenchmarkResult, error) {
	return m.runBenchmarks(ctx, false)
}

// BenchmarkSummary measures the wall-clock time of key pipeline stages
// and returns a human-readable table. This is a convenience wrapper
// around [Go.Benchmark] for CLI use without jq post-processing.
//
// When parallel is true, all stages run concurrently to measure the
// real-world wall-clock time of the full CI pipeline. The total row
// shows overall elapsed time rather than the sum of individual stages.
//
// +cache="never"
func (m *Go) BenchmarkSummary(
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

// CacheBust returns a container with a unique cache-busting environment
// variable that forces Dagger to re-evaluate the pipeline instead of
// returning cached results.
func (m *Go) CacheBust(
	// Container to bust the cache for.
	ctr *dagger.Container,
) *dagger.Container {
	return ctr.WithEnvVariable("_BENCH_TS", time.Now().String())
}

// benchmarkStage pairs a stage name with its execution function.
type benchmarkStage struct {
	name string
	fn   func(context.Context) error
}

// benchmarkStages returns the list of generic pipeline stages to benchmark.
func (m *Go) benchmarkStages() []benchmarkStage {
	return []benchmarkStage{
		{"goBase", func(ctx context.Context) error {
			_, err := m.CacheBust(m.GoBase()).Sync(ctx)
			return err
		}},
		{"lint", func(ctx context.Context) error {
			_, err := m.CacheBust(m.LintBase()).
				WithExec([]string{"golangci-lint", "run"}).
				Sync(ctx)
			return err
		}},
		{"test", func(ctx context.Context) error {
			_, err := m.CacheBust(m.GoBase()).
				WithExec([]string{"go", "test", "./..."}).
				Sync(ctx)
			return err
		}},
		{"lint-prettier", func(ctx context.Context) error {
			_, err := m.CacheBust(m.PrettierBase()).
				WithMountedDirectory("/src", m.Source).
				WithWorkdir("/src").
				WithExec(append(
					[]string{"prettier", "--config", "./.prettierrc.yaml", "--check"},
					defaultPrettierPatterns()...,
				)).
				Sync(ctx)
			return err
		}},
		{"lint-actions", func(ctx context.Context) error {
			_, err := m.CacheBust(dag.Container().
				From("ghcr.io/zizmorcore/zizmor:"+zizmorVersion)).
				WithMountedDirectory("/src", m.Source).
				WithWorkdir("/src").
				WithExec([]string{
					"zizmor", ".github/workflows", "--config", ".github/zizmor.yaml",
				}).
				Sync(ctx)
			return err
		}},
		{"lint-releaser", func(ctx context.Context) error {
			_, err := m.CacheBust(m.GoreleaserCheckBase("")).
				WithExec([]string{"goreleaser", "check"}).
				Sync(ctx)
			return err
		}},
	}
}

// runBenchmarks executes benchmark stages. When parallel is false, stages
// run sequentially for isolated timings. When true, stages run concurrently
// to measure real-world wall-clock time.
func (m *Go) runBenchmarks(ctx context.Context, parallel bool) ([]*BenchmarkResult, error) {
	stages := m.benchmarkStages()

	if parallel {
		return m.runBenchmarksParallel(ctx, stages)
	}
	return m.runBenchmarksSequential(ctx, stages)
}

// runBenchmarksSequential runs each stage one at a time for isolated timings.
func (m *Go) runBenchmarksSequential(ctx context.Context, stages []benchmarkStage) ([]*BenchmarkResult, error) {
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
func (m *Go) runBenchmarksParallel(ctx context.Context, stages []benchmarkStage) ([]*BenchmarkResult, error) {
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
// Formatting
// ---------------------------------------------------------------------------

// Format runs golangci-lint --fix and prettier --write, returning the
// changeset against the original source directory.
//
// Both formatters operate on non-overlapping file types (.go vs
// .yaml/.md/.json), so they run against the original source in parallel.
// The results are merged by overlaying Prettier's output onto the
// Go-formatted source.
func (m *Go) Format() *dagger.Changeset {
	patterns := defaultPrettierPatterns()

	// Go formatting via golangci-lint --fix.
	goFmt := m.LintBase().
		WithExec([]string{"golangci-lint", "run", "--fix"}).
		Directory("/src")

	// Prettier formatting (runs against original source in parallel with Go formatting).
	prettierFmt := m.PrettierBase().
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec(append(
			[]string{"prettier", "--config", "./.prettierrc.yaml", "-w"},
			patterns...,
		)).
		Directory("/src")

	// Merge: start with Go-formatted source, overlay Prettier-formatted files.
	// Dagger evaluates lazily, so both pipelines execute concurrently when the
	// changeset is resolved.
	formatted := goFmt.WithDirectory(".", prettierFmt, dagger.DirectoryWithDirectoryOpts{
		Include: patterns,
	})

	return formatted.Changes(m.Source)
}

// ---------------------------------------------------------------------------
// Generation
// ---------------------------------------------------------------------------

// Generate runs go generate and returns the changeset of generated files
// against the original source.
func (m *Go) Generate() *dagger.Changeset {
	generated := m.GoBase().
		WithExec([]string{"go", "generate", "./..."}).
		Directory("/src").
		WithoutDirectory(".git")
	return generated.Changes(m.Source)
}

// ---------------------------------------------------------------------------
// Publishing
// ---------------------------------------------------------------------------

// VersionTags returns the image tags derived from a version tag string.
// For example, "v1.2.3" yields ["latest", "v1.2.3", "v1", "v1.2"].
func (m *Go) VersionTags(
	// Version tag (e.g. "v1.2.3").
	tag string,
) []string {
	v := strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(v, ".", 3)

	// Detect pre-release: any version component contains a hyphen
	// (e.g. "1.0.0-rc.1" -> third part is "0-rc.1").
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

// FormatDigestChecksums converts publish output references to the
// checksums format expected by actions/attest-build-provenance. Each reference
// has the form "registry/image:tag@sha256:hex"; this function emits
// "hex  registry/image:tag" lines, deduplicating by digest.
func (m *Go) FormatDigestChecksums(
	// Image references (e.g. "registry/image:tag@sha256:hex").
	refs []string,
) string {
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

// DeduplicateDigests returns unique image references from a list, keeping
// only the first occurrence of each sha256 digest.
func (m *Go) DeduplicateDigests(
	// Image references (e.g. "registry/image:tag@sha256:hex").
	refs []string,
) []string {
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

// RegistryHost extracts the host (with optional port) from a registry
// address. For example, "ghcr.io/macropower/kclipper" returns "ghcr.io".
func (m *Go) RegistryHost(
	// Registry address (e.g. "ghcr.io/macropower/kclipper").
	registry string,
) string {
	return strings.SplitN(registry, "/", 2)[0]
}

// PublishImages publishes pre-built container image variants to the registry,
// optionally signing them with cosign. Returns the list of published digest
// references (one per tag, e.g. "registry/image:tag@sha256:hex").
//
// +cache="never"
func (m *Go) PublishImages(
	ctx context.Context,
	// Pre-built container image variants to publish.
	variants []*dagger.Container,
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
) ([]string, error) {
	// Publish multi-arch manifest for each tag concurrently.
	publisher := dag.Container()
	if registryPassword != nil {
		publisher = publisher.WithRegistryAuth(m.RegistryHost(m.Registry), registryUsername, registryPassword)
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
		toSign := m.DeduplicateDigests(digests)

		cosignCtr := dag.Container().
			From("gcr.io/projectsigstore/cosign:"+cosignVersion).
			WithSecretVariable("COSIGN_KEY", cosignKey)
		if registryPassword != nil {
			cosignCtr = cosignCtr.WithRegistryAuth(m.RegistryHost(m.Registry), registryUsername, registryPassword)
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

// ---------------------------------------------------------------------------
// Base containers (public)
// ---------------------------------------------------------------------------

// GoBase returns a Go container with source, module cache, and build cache.
// A static .git/HEAD file is injected into the source so that tools
// can locate the repository root without a container exec. Module download
// is cached via [Go.GoModBase].
func (m *Go) GoBase() *dagger.Container {
	src := m.Source.WithNewFile(".git/HEAD", "ref: refs/heads/main\n")
	ctr := dag.Container().
		From("golang:"+goVersion).
		WithEnvVariable("CGO_ENABLED", "1")
	return m.GoModBase(ctr, src)
}

// LintBase returns a golangci-lint container with source and caches. The
// Debian-based image is used (not Alpine) because it includes kernel headers
// needed by CGO transitive dependencies. Module download is cached via
// [Go.GoModBase]. The golangci-lint cache volume includes the linter
// version so that version bumps start fresh.
func (m *Go) LintBase() *dagger.Container {
	ctr := dag.Container().
		From("golangci/golangci-lint:" + golangciLintVersion)
	return m.GoModBase(ctr, nil).
		WithMountedCache("/root/.cache/golangci-lint", dag.CacheVolume("golangci-lint-"+golangciLintVersion))
}

// PrettierBase returns a Node container with prettier pre-installed.
// Callers must mount their source directory and set the workdir.
func (m *Go) PrettierBase() *dagger.Container {
	return dag.Container().
		From("node:lts-slim").
		WithMountedCache("/root/.npm", dag.CacheVolume("npm-cache")).
		WithExec([]string{"npm", "install", "-g", "prettier@" + prettierVersion})
}

// GoModBase mounts Go module and build cache volumes, copies [Go.GoMod]
// (go.mod and go.sum only) into the container, and runs go mod download.
// Because [Go.GoMod] is synced independently of [Go.Source], its content
// hash changes only when dependency files change, not on every source edit.
// The cache volumes are mounted before the download so that go mod download
// is a near-instant no-op when modules are already present in the persistent
// volume. The full source directory is mounted last. Both [Go.LintBase]
// and [Go.GoBase] delegate to this method.
//
// Cache volumes include the Go version suffix (e.g. "go-mod-1.25") so that
// a Go version bump automatically starts with a fresh cache instead of
// inheriting potentially incompatible artifacts from the previous version.
func (m *Go) GoModBase(
	// Base container to add module caches and source to.
	ctr *dagger.Container,
	// Source directory to mount after module download. Defaults to the
	// module's [Go.Source].
	// +optional
	src *dagger.Directory,
) *dagger.Container {
	if src == nil {
		src = m.Source
	}
	return ctr.
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod-"+goVersion)).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build-"+goVersion)).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithDirectory("/src", m.GoMod).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"}).
		WithMountedDirectory("/src", src)
}

// GoreleaserCheckBase returns a lightweight container with only Go,
// GoReleaser, and the project source. This is sufficient for
// goreleaser check which only validates config syntax.
func (m *Go) GoreleaserCheckBase(
	// Remote URL to configure as origin. Some GoReleaser configs
	// reference a git remote; pass the repo URL when needed.
	// +optional
	remoteURL string,
) *dagger.Container {
	ctr := dag.Container().
		From("golang:"+goVersion).
		// Install GoReleaser from its official OCI image.
		WithFile("/usr/local/bin/goreleaser",
			dag.Container().From("ghcr.io/goreleaser/goreleaser:"+goreleaserVersion).
				File("/usr/bin/goreleaser"))
	return m.EnsureGitRepo(m.GoModBase(ctr, nil), remoteURL)
}

// ---------------------------------------------------------------------------
// Helpers (public)
// ---------------------------------------------------------------------------

// EnsureGitInit ensures the container has a minimal .git directory at its
// working directory. This is sufficient for tools that only need to locate
// the repository root but do not inspect commit history or the index.
// Prefer [Go.EnsureGitRepo] when the tool requires committed files.
func (m *Go) EnsureGitInit(
	// Container to initialize.
	ctr *dagger.Container,
) *dagger.Container {
	return ctr.WithExec([]string{
		"sh", "-c",
		"if ! git rev-parse --git-dir >/dev/null 2>&1; then " +
			"rm -f .git && " +
			"git init -q; " +
			"fi",
	})
}

// EnsureGitRepo ensures the container has a valid git repository at its
// working directory with all files staged and committed. When running from
// a git worktree, the .git file references a host path that doesn't exist
// in the container. In that case, a full git repository is initialized so
// that tools like GoReleaser that depend on committed files, dirty-tree
// detection, and version derivation continue to work.
func (m *Go) EnsureGitRepo(
	// Container to initialize.
	ctr *dagger.Container,
	// Remote URL to add as origin. When empty, no remote is configured.
	// +optional
	remoteURL string,
) *dagger.Container {
	remoteCmd := ""
	if remoteURL != "" {
		remoteCmd = "git remote add origin " + remoteURL + " && "
	}
	return ctr.WithExec([]string{
		"sh", "-c",
		"if ! git rev-parse --git-dir >/dev/null 2>&1; then " +
			"rm -f .git && " +
			"git init -q && " +
			remoteCmd +
			"git add -A && " +
			"GIT_COMMITTER_DATE='2000-01-01T00:00:00+00:00' " +
			"git -c user.email=ci@dagger -c user.name=ci commit -q --allow-empty -m init " +
			"--date='2000-01-01T00:00:00+00:00'; " +
			"fi",
	})
}

// defaultPrettierPatterns returns the default file patterns for prettier
// formatting and linting.
func defaultPrettierPatterns() []string {
	return []string{
		"*.yaml", "*.md", "*.json",
		"**/*.yaml", "**/*.md", "**/*.json",
	}
}
