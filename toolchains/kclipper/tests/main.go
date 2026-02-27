// Integration tests for the [Kclipper] module. Individual tests are annotated
// with +check so `dagger check -m toolchains/kclipper/tests` runs them all concurrently.
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

// Tests provides integration tests for the [Kclipper] module. Create instances
// with [New].
type Tests struct{}

// All runs all tests in parallel.
func (m *Tests) All(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return m.TestBuildDist(ctx) })
	g.Go(func() error { return m.TestBuildImageMetadata(ctx) })
	g.Go(func() error { return m.TestLintReleaserClean(ctx) })
	g.Go(func() error { return m.TestLintDeadcodeClean(ctx) })

	return g.Wait()
}

// TestBuildDist verifies that [Kclipper.Build] returns a dist directory containing
// expected entries (checksums and at least one platform archive).
//
// +check
func (m *Tests) TestBuildDist(ctx context.Context) error {
	entries, err := dag.Kclipper().Build().Entries(ctx)
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

// TestBuildImageMetadata verifies that [Kclipper.BuildImages] produces containers
// with expected OCI labels, environment variables, and entrypoint.
//
// +check
func (m *Tests) TestBuildImageMetadata(ctx context.Context) error {
	dist := dag.Kclipper().Build()
	variants, err := dag.Kclipper().BuildImages(ctx, dagger.KclipperBuildImagesOpts{
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

// TestPublishImages verifies that [Kclipper.PublishImages] builds, publishes,
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
//	dagger call -m toolchains/kclipper/tests test-publish-images
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
	ci := dag.Kclipper(dagger.KclipperOpts{Registry: registry})

	// Publish 2 tags to exercise deduplication (both tags share one manifest digest).
	dist := ci.Build()
	result, err := ci.PublishImages(ctx, []string{"1h", "2h"}, dagger.KclipperPublishImagesOpts{
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

// TestLintReleaserClean verifies that the GoReleaser configuration passes
// validation. This exercises the [Kclipper.LintReleaser] check, which requires
// the kclipper git remote for homebrew/nix repository resolution.
//
// +check
func (m *Tests) TestLintReleaserClean(ctx context.Context) error {
	return dag.Kclipper().LintReleaser(ctx)
}

// TestLintDeadcodeClean verifies that the codebase has no unreachable
// functions. This exercises [Go.LintDeadcode].
func (m *Tests) TestLintDeadcodeClean(ctx context.Context) error {
	return dag.Go().LintDeadcode(ctx)
}

// TestBenchmarkReturnsResults verifies that [Kclipper.Benchmark] returns
// non-empty results with expected stage names and positive durations.
//
// Not annotated with +check because benchmarks run the full pipeline
// with cache-busting, which would duplicate all CI work in the
// integration test suite. Run manually:
//
//	dagger call -m toolchains/kclipper/tests test-benchmark-returns-results
func (m *Tests) TestBenchmarkReturnsResults(ctx context.Context) error {
	results, err := dag.Kclipper().Benchmark(ctx)
	if err != nil {
		return fmt.Errorf("run benchmark: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("benchmark returned no results")
	}

	expectedStages := map[string]bool{
		"goBase":        false,
		"lint":          false,
		"test":          false,
		"lint-prettier": false,
		"lint-actions":  false,
		"lint-releaser": false,
		"build":         false,
	}

	for _, r := range results {
		name, err := r.Name(ctx)
		if err != nil {
			return fmt.Errorf("read result name: %w", err)
		}
		if _, ok := expectedStages[name]; ok {
			expectedStages[name] = true
		}

		dur, err := r.DurationSecs(ctx)
		if err != nil {
			return fmt.Errorf("read duration for %s: %w", name, err)
		}
		if dur < 0 {
			return fmt.Errorf("stage %s has negative duration: %f", name, dur)
		}

		ok, err := r.Ok(ctx)
		if err != nil {
			return fmt.Errorf("read ok for %s: %w", name, err)
		}
		if !ok {
			errMsg, _ := r.Error(ctx)
			return fmt.Errorf("stage %s failed: %s", name, errMsg)
		}
	}

	for stage, found := range expectedStages {
		if !found {
			return fmt.Errorf("expected stage %q not found in results", stage)
		}
	}

	return nil
}

// TestBenchmarkSummaryFormat verifies that [Kclipper.BenchmarkSummary] returns
// a non-empty string containing the expected table header and stage names.
//
// Not annotated with +check because benchmarks run the full pipeline
// with cache-busting (see [Tests.TestBenchmarkReturnsResults]). Run manually:
//
//	dagger call -m toolchains/kclipper/tests test-benchmark-summary-format
func (m *Tests) TestBenchmarkSummaryFormat(ctx context.Context) error {
	summary, err := dag.Kclipper().BenchmarkSummary(ctx)
	if err != nil {
		return fmt.Errorf("run benchmark summary: %w", err)
	}
	if len(summary) == 0 {
		return fmt.Errorf("benchmark summary is empty")
	}

	// Verify the table header is present.
	if !strings.Contains(summary, "STAGE") || !strings.Contains(summary, "DURATION") {
		return fmt.Errorf("benchmark summary missing table header: %s", summary)
	}

	// Verify key stages appear in the output.
	for _, stage := range []string{"goBase", "lint", "test", "build"} {
		if !strings.Contains(summary, stage) {
			return fmt.Errorf("benchmark summary missing stage %q: %s", stage, summary)
		}
	}

	// Verify the total row is present.
	if !strings.Contains(summary, "TOTAL") {
		return fmt.Errorf("benchmark summary missing TOTAL row: %s", summary)
	}

	return nil
}
