// Integration tests for the [Ci] module. Individual tests are annotated
// with +check so `dagger check -m ci/tests` runs them all concurrently.

package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/tests/internal/dagger"

	"golang.org/x/sync/errgroup"
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

	excluded := []string{"dist", ".worktrees", ".tmp", ".devcontainer"}
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
