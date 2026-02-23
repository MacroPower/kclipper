// Integration tests for the [Ci] module. Individual tests are annotated
// with +check so `dagger check -m ci/tests` runs them all concurrently.
//
// Security invariant: no test in this module should use
// InsecureRootCapabilities or ExperimentalPrivilegedNesting.
// These options bypass container sandboxing and are only appropriate
// for interactive use (Dev terminal). Adding either to a test
// function requires explicit security review justification.

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/tests/internal/dagger"

	"golang.org/x/sync/errgroup"
)

const (
	cosignVersion = "v3.0.4" // renovate: datasource=github-releases depName=sigstore/cosign
)

// Tests provides integration tests for the [Ci] module. Create instances
// with [New].
type Tests struct{}

// All runs all tests in parallel.
func (m *Tests) All(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return m.TestSourceFiltering(ctx) })
	g.Go(func() error { return m.TestFormatIdempotent(ctx) })
	g.Go(func() error { return m.TestLintActionsClean(ctx) })
	g.Go(func() error { return m.TestVersionTags(ctx) })
	g.Go(func() error { return m.TestBuildDist(ctx) })
	g.Go(func() error { return m.TestBuildImageMetadata(ctx) })
	g.Go(func() error { return m.TestFormatDigestChecksums(ctx) })
	g.Go(func() error { return m.TestDeduplicateDigests(ctx) })
	g.Go(func() error { return m.TestRegistryHost(ctx) })
	g.Go(func() error { return m.TestLintReleaserClean(ctx) })
	g.Go(func() error { return m.TestDevBase(ctx) })
	g.Go(func() error { return m.TestDevExportPersistence(ctx) })
	g.Go(func() error { return m.TestCoverageProfile(ctx) })

	return g.Wait()
}

// TestSourceFiltering verifies that the +ignore annotation in [Ci.New]
// excludes the expected directories from the source.
//
// +check
func (m *Tests) TestSourceFiltering(ctx context.Context) error {
	entries, err := dag.Ci().Source().Entries(ctx)
	if err != nil {
		return fmt.Errorf("list source entries: %w", err)
	}

	excluded := []string{"dist", ".worktrees", ".tmp"}
	for _, dir := range excluded {
		for _, entry := range entries {
			if strings.TrimRight(entry, "/") == dir {
				return fmt.Errorf("source should exclude %q but it was present", dir)
			}
		}
	}

	// Verify essential files are present.
	required := []string{"go.mod", "ci"}
	for _, name := range required {
		found := false
		for _, entry := range entries {
			if strings.TrimRight(entry, "/") == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("source should include %q but it was missing (entries: %v)", name, entries)
		}
	}

	return nil
}

// TestFormatIdempotent verifies that running the formatter on already-formatted
// source produces an empty changeset. This exercises the full [Ci.Format]
// pipeline (golangci-lint --fix + prettier --write) and confirms the source is
// clean.
//
// +check
func (m *Tests) TestFormatIdempotent(ctx context.Context) error {
	changeset := dag.Ci().Format()

	empty, err := changeset.IsEmpty(ctx)
	if err != nil {
		return fmt.Errorf("check changeset: %w", err)
	}
	if !empty {
		modified, _ := changeset.ModifiedPaths(ctx)
		added, _ := changeset.AddedPaths(ctx)
		removed, _ := changeset.RemovedPaths(ctx)
		return fmt.Errorf("expected empty changeset on clean source, modified=%v added=%v removed=%v",
			modified, added, removed)
	}
	return nil
}

// TestLintActionsClean verifies that the GitHub Actions workflows pass
// zizmor linting. This exercises the [Ci.LintActions] check and catches
// workflow security or syntax issues.
//
// +check
func (m *Tests) TestLintActionsClean(ctx context.Context) error {
	return dag.Ci().LintActions(ctx)
}

// TestVersionTags verifies that [Ci.VersionTags] returns the expected set of
// image tags for various version strings.
//
// +check
func (m *Tests) TestVersionTags(ctx context.Context) error {
	cases := map[string]struct {
		tag  string
		want []string
	}{
		"semver": {
			tag:  "v1.2.3",
			want: []string{"latest", "v1.2.3", "v1", "v1.2"},
		},
		"pre-release": {
			tag:  "v0.5.1-rc.1",
			want: []string{"v0.5.1-rc.1"},
		},
		"two-component": {
			tag:  "v2.0",
			want: []string{"latest", "v2.0", "v2", "v2.0"},
		},
		"single-component": {
			tag:  "v1",
			want: []string{"latest", "v1", "v1"},
		},
		"four-component": {
			tag:  "v1.2.3.4",
			want: []string{"latest", "v1.2.3.4", "v1", "v1.2"},
		},
		"empty-string": {
			tag:  "",
			want: []string{"latest", "", "v"},
		},
		"no-v-prefix": {
			tag:  "1.2.3",
			want: []string{"latest", "1.2.3", "v1", "v1.2"},
		},
		"hyphen-in-first-component": {
			tag:  "v0-beta.1",
			want: []string{"v0-beta.1"},
		},
	}

	for name, tc := range cases {
		got, err := dag.Ci().VersionTags(ctx, tc.tag)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if len(got) != len(tc.want) {
			return fmt.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				return fmt.Errorf("%s: index %d: got %q, want %q", name, i, got[i], tc.want[i])
			}
		}
	}

	return nil
}

// TestBuildDist verifies that [Ci.Build] returns a dist directory containing
// expected entries (checksums and at least one platform archive).
//
// +check
func (m *Tests) TestBuildDist(ctx context.Context) error {
	entries, err := dag.Ci().Build().Entries(ctx)
	if err != nil {
		return fmt.Errorf("list dist entries: %w", err)
	}

	hasChecksums := false
	hasArchive := false
	for _, entry := range entries {
		if strings.Contains(entry, "checksums") {
			hasChecksums = true
		}
		if strings.Contains(entry, "linux_amd64") || strings.Contains(entry, "linux_arm64") {
			hasArchive = true
		}
	}

	if !hasChecksums {
		return fmt.Errorf("dist missing checksums file (entries: %v)", entries)
	}
	if !hasArchive {
		return fmt.Errorf("dist missing platform archive (entries: %v)", entries)
	}

	return nil
}

// TestBuildImageMetadata verifies that [Ci.BuildImages] produces containers
// with expected OCI labels, environment variables, and entrypoint.
//
// +check
func (m *Tests) TestBuildImageMetadata(ctx context.Context) error {
	dist := dag.Ci().Build()
	variants, err := dag.Ci().BuildImages(ctx, dagger.CiBuildImagesOpts{
		Version: "v0.0.0-test",
		Dist:    dist,
	})
	if err != nil {
		return fmt.Errorf("build images: %w", err)
	}
	if len(variants) != 2 {
		return fmt.Errorf("expected 2 image variants (linux/amd64, linux/arm64), got %d", len(variants))
	}

	for i, ctr := range variants {
		// Verify OCI version label.
		version, err := ctr.Label(ctx, "org.opencontainers.image.version")
		if err != nil {
			return fmt.Errorf("variant %d: version label: %w", i, err)
		}
		if version != "v0.0.0-test" {
			return fmt.Errorf("variant %d: version label = %q, want %q", i, version, "v0.0.0-test")
		}

		// Verify OCI title label.
		title, err := ctr.Label(ctx, "org.opencontainers.image.title")
		if err != nil {
			return fmt.Errorf("variant %d: title label: %w", i, err)
		}
		if title != "kclipper" {
			return fmt.Errorf("variant %d: title label = %q, want %q", i, title, "kclipper")
		}

		// Verify OCI created label is present and non-empty.
		created, err := ctr.Label(ctx, "org.opencontainers.image.created")
		if err != nil {
			return fmt.Errorf("variant %d: created label: %w", i, err)
		}
		if created == "" {
			return fmt.Errorf("variant %d: created label is empty", i)
		}

		// Verify entrypoint.
		ep, err := ctr.Entrypoint(ctx)
		if err != nil {
			return fmt.Errorf("variant %d: entrypoint: %w", i, err)
		}
		if len(ep) != 1 || ep[0] != "kcl" {
			return fmt.Errorf("variant %d: entrypoint = %v, want [kcl]", i, ep)
		}

		// Verify KCL environment variable.
		fastEval, err := ctr.EnvVariable(ctx, "KCL_FAST_EVAL")
		if err != nil {
			return fmt.Errorf("variant %d: KCL_FAST_EVAL: %w", i, err)
		}
		if fastEval != "1" {
			return fmt.Errorf("variant %d: KCL_FAST_EVAL = %q, want %q", i, fastEval, "1")
		}
	}

	return nil
}

// TestPublishImages verifies that [Ci.PublishImages] builds, publishes,
// signs, and produces verifiable cosign signatures. Uses ttl.sh as an
// anonymous temporary registry (images expire after the tag duration).
//
// The test publishes 2 tags to exercise the digest deduplication path
// (both tags share one manifest digest, so cosign signs only once). An
// ephemeral cosign key pair is generated per run; the signature is
// verified with the public key after publishing.
//
// Not annotated with +check because it depends on external network access
// to ttl.sh and takes ~5 minutes. Run manually:
//
//	dagger call -m ci/tests test-publish-images
func (m *Tests) TestPublishImages(ctx context.Context) error {
	// Generate an ephemeral cosign key pair for signing and verification.
	cosignCtr := dag.Container().
		From("gcr.io/projectsigstore/cosign:"+cosignVersion).
		WithEnvVariable("COSIGN_PASSWORD", "test-password").
		WithExec([]string{"cosign", "generate-key-pair"})
	privKeyContent, err := cosignCtr.File("cosign.key").Contents(ctx)
	if err != nil {
		return fmt.Errorf("generate cosign key pair: %w", err)
	}
	pubKey := cosignCtr.File("cosign.pub")
	cosignKey := dag.SetSecret("test-cosign-key", privKeyContent)
	cosignPassword := dag.SetSecret("test-cosign-password", "test-password")

	// Use a unique registry path on ttl.sh to avoid collisions between runs.
	registry := fmt.Sprintf("ttl.sh/kclipper-ci-%d", time.Now().UnixNano())
	ci := dag.Ci(dagger.CiOpts{Registry: registry})

	// Publish 2 tags to exercise deduplication (both tags share one manifest digest).
	dist := ci.Build()
	result, err := ci.PublishImages(ctx, []string{"1h", "2h"}, dagger.CiPublishImagesOpts{
		Dist:           dist,
		CosignKey:      cosignKey,
		CosignPassword: cosignPassword,
	})
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	// Verify the result contains sha256 digest references.
	if !strings.Contains(result, "sha256:") {
		return fmt.Errorf("expected sha256 digest in result, got: %s", result)
	}

	// Verify 2 tags were published.
	if !strings.Contains(result, "published 2 tags") {
		return fmt.Errorf("expected 'published 2 tags' in result, got: %s", result)
	}

	// Verify deduplication: both tags share one manifest, so 1 unique digest.
	if !strings.Contains(result, "1 unique digests") {
		return fmt.Errorf("expected '1 unique digests' in result, got: %s", result)
	}

	// Extract a digest reference for signature verification.
	// Result format: "published 2 tags (1 unique digests)\nregistry:tag@sha256:hex\n..."
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("expected at least 2 lines in result, got %d: %s", len(lines), result)
	}
	digestRef := lines[1]
	if !strings.Contains(digestRef, "@sha256:") {
		return fmt.Errorf("expected digest reference in line 1, got: %s", digestRef)
	}

	// Verify the cosign signature using the ephemeral public key.
	// --insecure-ignore-tlog=true skips Rekor transparency log verification
	// to avoid flakiness; core cryptographic signature verification still runs.
	_, err = dag.Container().
		From("gcr.io/projectsigstore/cosign:"+cosignVersion).
		WithMountedFile("/cosign.pub", pubKey).
		WithExec([]string{
			"cosign", "verify",
			"--key", "/cosign.pub",
			"--insecure-ignore-tlog=true",
			digestRef,
		}).
		Sync(ctx)
	if err != nil {
		return fmt.Errorf("verify cosign signature: %w", err)
	}

	return nil
}

// TestFormatDigestChecksums verifies that [Ci.FormatDigestChecksums] converts
// publish output to the checksums format, deduplicating by digest.
//
// +check
func (m *Tests) TestFormatDigestChecksums(ctx context.Context) error {
	refs := []string{
		"ghcr.io/test:v1@sha256:abc123",
		"ghcr.io/test:v2@sha256:abc123", // duplicate digest
		"ghcr.io/test:latest@sha256:def456",
	}

	result, err := dag.Ci().FormatDigestChecksums(ctx, refs)
	if err != nil {
		return fmt.Errorf("format: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		return fmt.Errorf("expected 2 lines (deduplicated), got %d: %q", len(lines), result)
	}

	if lines[0] != "abc123  ghcr.io/test:v1" {
		return fmt.Errorf("line 0 = %q, want %q", lines[0], "abc123  ghcr.io/test:v1")
	}
	if lines[1] != "def456  ghcr.io/test:latest" {
		return fmt.Errorf("line 1 = %q, want %q", lines[1], "def456  ghcr.io/test:latest")
	}

	return nil
}

// TestDeduplicateDigests verifies that [Ci.DeduplicateDigests] keeps only the
// first occurrence of each sha256 digest.
//
// +check
func (m *Tests) TestDeduplicateDigests(ctx context.Context) error {
	refs := []string{
		"ghcr.io/test:v1@sha256:abc123",
		"ghcr.io/test:latest@sha256:abc123",
		"ghcr.io/test:v1.0@sha256:def456",
	}

	result, err := dag.Ci().DeduplicateDigests(ctx, refs)
	if err != nil {
		return fmt.Errorf("deduplicate: %w", err)
	}

	if len(result) != 2 {
		return fmt.Errorf("expected 2 unique refs, got %d: %v", len(result), result)
	}
	if result[0] != "ghcr.io/test:v1@sha256:abc123" {
		return fmt.Errorf("ref 0 = %q, want %q", result[0], "ghcr.io/test:v1@sha256:abc123")
	}
	if result[1] != "ghcr.io/test:v1.0@sha256:def456" {
		return fmt.Errorf("ref 1 = %q, want %q", result[1], "ghcr.io/test:v1.0@sha256:def456")
	}

	return nil
}

// TestRegistryHost verifies that [Ci.RegistryHost] extracts the host (with
// optional port) from various registry address formats.
//
// +check
func (m *Tests) TestRegistryHost(ctx context.Context) error {
	cases := map[string]struct {
		registry string
		want     string
	}{
		"standard-registry": {
			registry: "ghcr.io/macropower/kclipper",
			want:     "ghcr.io",
		},
		"with-port": {
			registry: "localhost:5000/myimage",
			want:     "localhost:5000",
		},
		"host-only": {
			registry: "docker.io",
			want:     "docker.io",
		},
		"nested-path": {
			registry: "registry.example.com/org/team/image",
			want:     "registry.example.com",
		},
		"empty-string": {
			registry: "",
			want:     "",
		},
	}

	for name, tc := range cases {
		got, err := dag.Ci().RegistryHost(ctx, tc.registry)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if got != tc.want {
			return fmt.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}

	return nil
}

// TestLintReleaserClean verifies that the GoReleaser configuration passes
// validation. This exercises the [Ci.LintReleaser] check.
//
// +check
func (m *Tests) TestLintReleaserClean(ctx context.Context) error {
	return dag.Ci().LintReleaser(ctx)
}

// TestDevBase verifies that [devBase] produces a container with essential
// development tools available on PATH. This validates the tool installation
// pipeline without requiring an interactive terminal session.
//
// +check
func (m *Tests) TestDevBase(ctx context.Context) error {
	// Dev() returns a Directory (interactive terminal + export), so we
	// test the underlying devBase container via a non-interactive build.
	ctr := dag.Ci().DevBase()

	tools := []string{
		"go", "task", "dagger", "conform", "lefthook", "claude",
		"starship", "yq", "uv", "gh", "direnv",
		"rg", "fd", "bat", "fzf", "tree", "htop",
	}
	for _, tool := range tools {
		_, err := ctr.WithExec([]string{"which", tool}).Sync(ctx)
		if err != nil {
			return fmt.Errorf("%s not found in dev container: %w", tool, err)
		}
	}

	return nil
}

// TestDevExportPersistence verifies that the [Ci.Dev] export pipeline
// correctly captures new files, preserves original source files, and
// excludes the .git directory. This reconstructs the same cache-volume
// pipeline that [Ci.Dev] uses but replaces [dagger.Container.Terminal]
// with [dagger.Container.WithExec] to simulate interactive changes.
//
// +check
func (m *Tests) TestDevExportPersistence(ctx context.Context) error {
	// Reconstruct the Dev() pipeline without Terminal().
	// Use a test-specific cache volume to avoid interfering with real
	// dev sessions.
	exported := dag.Ci().DevBase().
		WithDirectory("/tmp/src-seed", dag.Ci().Source()).
		WithMountedCache("/src", dag.CacheVolume("test-dev-src")).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithWorkdir("/src").
		// Seed the cache volume with fresh source.
		WithEnvVariable("_TEST_TS", time.Now().String()).
		WithExec([]string{"sh", "-c", "rm -rf /src/.git && cp -a /tmp/src-seed/. /src/"}).
		// Ensure git repo exists (same as Dev pipeline).
		WithExec([]string{"sh", "-c",
			"if ! git rev-parse --git-dir >/dev/null 2>&1; then "+
				"rm -f .git && git init -q && "+
				"git add -A && "+
				"git -c user.email=ci@dagger -c user.name=ci commit -q --allow-empty -m init; fi",
		}).
		// Simulate interactive changes: create a new file + modify an existing one.
		WithExec([]string{"sh", "-c",
			"echo test-content > /src/test-sentinel && "+
				"echo '# modified' >> /src/README.md",
		}).
		// Copy to output (same as Dev pipeline).
		WithExec([]string{"sh", "-c", "mkdir -p /output && cp -a /src/. /output/"}).
		Directory("/output").
		WithoutDirectory(".git")

	// Verify new file is captured.
	content, err := exported.File("test-sentinel").Contents(ctx)
	if err != nil {
		return fmt.Errorf("read sentinel file: %w", err)
	}
	if strings.TrimSpace(content) != "test-content" {
		return fmt.Errorf("sentinel content = %q, want %q", strings.TrimSpace(content), "test-content")
	}

	// Verify original source files are preserved.
	entries, err := exported.Entries(ctx)
	if err != nil {
		return fmt.Errorf("list exported entries: %w", err)
	}
	foundGoMod := false
	foundGit := false
	for _, entry := range entries {
		name := strings.TrimRight(entry, "/")
		switch name {
		case "go.mod":
			foundGoMod = true
		case ".git":
			foundGit = true
		}
	}
	if !foundGoMod {
		return fmt.Errorf("go.mod not found in export (entries: %v)", entries)
	}

	// Verify .git is excluded from export.
	if foundGit {
		return fmt.Errorf(".git should be excluded from export but was present")
	}

	// Verify modified file has our change.
	readme, err := exported.File("README.md").Contents(ctx)
	if err != nil {
		return fmt.Errorf("read README.md: %w", err)
	}
	if !strings.HasSuffix(strings.TrimSpace(readme), "# modified") {
		return fmt.Errorf("README.md modification not captured (last 50 chars: %q)",
			readme[max(0, len(readme)-50):])
	}

	return nil
}

// TestCoverageProfile verifies that [Ci.TestCoverage] returns a non-empty
// Go coverage profile containing the expected "mode:" header line.
//
// +check
func (m *Tests) TestCoverageProfile(ctx context.Context) error {
	contents, err := dag.Ci().TestCoverage().Contents(ctx)
	if err != nil {
		return fmt.Errorf("read coverage profile: %w", err)
	}
	if len(contents) == 0 {
		return fmt.Errorf("coverage profile is empty")
	}
	if !strings.Contains(contents, "mode:") {
		return fmt.Errorf("coverage profile missing 'mode:' header (got %d bytes)", len(contents))
	}
	return nil
}
