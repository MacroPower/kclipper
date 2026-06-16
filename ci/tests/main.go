// Integration tests for the [Ci] module. Individual tests are annotated with
// +check so `dagger check -m ci/tests` runs them all concurrently.
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/tests/internal/dagger"

	"golang.org/x/sync/errgroup"
)

// Tests provides integration tests for the [Ci] module. Create instances with
// [New].
type Tests struct{}

// All runs all tests in parallel.
func (m *Tests) All(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return m.TestBuildDist(ctx) })
	g.Go(func() error { return m.TestBuildImageMetadata(ctx) })
	g.Go(func() error { return m.TestLintReleaserClean(ctx) })
	g.Go(func() error { return m.TestBinary(ctx) })
	g.Go(func() error { return m.TestLintActionsClean(ctx) })
	g.Go(func() error { return m.TestLintKCLModulesClean(ctx) })

	return g.Wait()
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

// TestPublishImages verifies that [Ci.PublishImages] builds and publishes
// multi-arch images to a registry. Uses ttl.sh as an anonymous temporary
// registry (images expire after the tag duration).
//
// Signing is not tested here because keyless cosign requires an OIDC identity
// token (e.g. from GitHub Actions). Signing is exercised during real releases.
//
// The test publishes 2 tags to exercise the digest deduplication path (both
// tags share one manifest digest).
//
// Not annotated with +check because it depends on external network access
// to ttl.sh and takes ~5 minutes. Run manually:
//
//	dagger call -m ci/tests test-publish-images
func (m *Tests) TestPublishImages(ctx context.Context) error {
	// Use a unique registry path on ttl.sh to avoid collisions between runs.
	registry := fmt.Sprintf("ttl.sh/kclipper-ci-%d", time.Now().UnixNano())
	ci := dag.Ci(dagger.CiOpts{Registry: registry})

	// Publish 2 tags to exercise deduplication (both tags share one manifest digest).
	dist := ci.Build()
	result, err := ci.PublishImages(ctx, []string{"1h", "2h"}, dagger.CiPublishImagesOpts{
		Dist: dist,
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

	// Verify a digest reference is present.
	// Result format: "published 2 tags (1 unique digests)\nregistry:tag@sha256:hex\n..."
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("expected at least 2 lines in result, got %d: %s", len(lines), result)
	}
	digestRef := lines[1]
	if !strings.Contains(digestRef, "@sha256:") {
		return fmt.Errorf("expected digest reference in line 1, got: %s", digestRef)
	}

	return nil
}

// TestLintReleaserClean verifies that the GoReleaser configuration passes
// validation. This exercises the [Ci.LintReleaser] check, which requires
// the kclipper git remote for homebrew/nix repository resolution.
//
// +check
func (m *Tests) TestLintReleaserClean(ctx context.Context) error {
	return dag.Ci().LintReleaser(ctx)
}

// TestBinary verifies that [Ci.Binary] compiles the kcl binary.
//
// +check
func (m *Tests) TestBinary(ctx context.Context) error {
	size, err := dag.Ci().Binary().Size(ctx)
	if err != nil {
		return fmt.Errorf("binary: %w", err)
	}
	if size == 0 {
		return fmt.Errorf("binary has zero size")
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

// TestLintKCLModulesClean verifies that all KCL modules under modules/ can be
// packaged correctly. This exercises the [Ci.LintKCLModules] check.
//
// +check
func (m *Tests) TestLintKCLModulesClean(ctx context.Context) error {
	return dag.Ci().LintKclmodules(ctx)
}
