// Integration tests for the [Go] module. Individual tests are annotated
// with +check so `dagger check -m toolchains/go/tests` runs them all concurrently.
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

	"golang.org/x/sync/errgroup"
)

// Tests provides integration tests for the [Go] module. Create instances
// with [New].
type Tests struct{}

// All runs all tests in parallel.
func (m *Tests) All(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return m.TestSourceFiltering(ctx) })
	g.Go(func() error { return m.TestFormatIdempotent(ctx) })
	g.Go(func() error { return m.TestLintActionsClean(ctx) })
	g.Go(func() error { return m.TestVersionTags(ctx) })
	g.Go(func() error { return m.TestFormatDigestChecksums(ctx) })
	g.Go(func() error { return m.TestDeduplicateDigests(ctx) })
	g.Go(func() error { return m.TestRegistryHost(ctx) })
	g.Go(func() error { return m.TestGenerateIdempotent(ctx) })
	g.Go(func() error { return m.TestCoverageProfile(ctx) })
	g.Go(func() error { return m.TestLintCommitMsg(ctx) })

	return g.Wait()
}

// TestSourceFiltering verifies that the +ignore annotation in [Go.New]
// excludes the expected directories from the source.
//
// +check
func (m *Tests) TestSourceFiltering(ctx context.Context) error {
	entries, err := dag.Go().Source().Entries(ctx)
	if err != nil {
		return fmt.Errorf("list source entries: %w", err)
	}

	excluded := []string{"dist", ".worktrees", ".tmp", ".git"}
	for _, dir := range excluded {
		for _, entry := range entries {
			if strings.TrimRight(entry, "/") == dir {
				return fmt.Errorf("source should exclude %q but it was present", dir)
			}
		}
	}

	// Verify essential files are present.
	required := []string{"go.mod", "toolchains"}
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
// source produces an empty changeset. This exercises the full [Go.Format]
// pipeline (golangci-lint --fix + prettier --write) and confirms the source is
// clean.
//
// +check
func (m *Tests) TestFormatIdempotent(ctx context.Context) error {
	changeset := dag.Go().Format()

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
// zizmor linting. This exercises the [Go.LintActions] check and catches
// workflow security or syntax issues.
//
// +check
func (m *Tests) TestLintActionsClean(ctx context.Context) error {
	return dag.Go().LintActions(ctx)
}

// TestVersionTags verifies that [Go.VersionTags] returns the expected set of
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
		got, err := dag.Go().VersionTags(ctx, tc.tag)
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

// TestFormatDigestChecksums verifies that [Go.FormatDigestChecksums] converts
// publish output to the checksums format, deduplicating by digest.
//
// +check
func (m *Tests) TestFormatDigestChecksums(ctx context.Context) error {
	refs := []string{
		"ghcr.io/test:v1@sha256:abc123",
		"ghcr.io/test:v2@sha256:abc123", // duplicate digest
		"ghcr.io/test:latest@sha256:def456",
	}

	result, err := dag.Go().FormatDigestChecksums(ctx, refs)
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

// TestDeduplicateDigests verifies that [Go.DeduplicateDigests] keeps only the
// first occurrence of each sha256 digest.
//
// +check
func (m *Tests) TestDeduplicateDigests(ctx context.Context) error {
	refs := []string{
		"ghcr.io/test:v1@sha256:abc123",
		"ghcr.io/test:latest@sha256:abc123",
		"ghcr.io/test:v1.0@sha256:def456",
	}

	result, err := dag.Go().DeduplicateDigests(ctx, refs)
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

// TestRegistryHost verifies that [Go.RegistryHost] extracts the host (with
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
		got, err := dag.Go().RegistryHost(ctx, tc.registry)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if got != tc.want {
			return fmt.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}

	return nil
}

// TestGenerateIdempotent verifies that running the generator on
// already-generated source produces an empty changeset. This exercises the
// full [Go.Generate] pipeline and confirms the source is clean.
//
// +check
func (m *Tests) TestGenerateIdempotent(ctx context.Context) error {
	changeset := dag.Go().Generate()

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

// TestLintCommitMsg verifies that [Go.LintCommitMsg] accepts valid
// conventional commit messages and rejects invalid ones.
//
// +check
func (m *Tests) TestLintCommitMsg(ctx context.Context) error {
	validMsg := dag.Directory().
		WithNewFile("COMMIT_EDITMSG", "feat(cli): add new flag for output format\n").
		File("COMMIT_EDITMSG")
	if err := dag.Go().LintCommitMsg(ctx, validMsg); err != nil {
		return fmt.Errorf("valid commit message rejected: %w", err)
	}

	invalidMsg := dag.Directory().
		WithNewFile("COMMIT_EDITMSG", "This is not a conventional commit.\n").
		File("COMMIT_EDITMSG")
	if err := dag.Go().LintCommitMsg(ctx, invalidMsg); err == nil {
		return fmt.Errorf("invalid commit message was not rejected")
	}

	return nil
}

// TestCoverageProfile verifies that [Go.TestCoverage] returns a non-empty
// Go coverage profile containing the expected "mode:" header line.
//
// +check
func (m *Tests) TestCoverageProfile(ctx context.Context) error {
	contents, err := dag.Go().TestCoverage().Contents(ctx)
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

