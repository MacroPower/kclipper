package main

import (
	"context"
	"fmt"

	"dagger/go/internal/dagger"
)

// ---------------------------------------------------------------------------
// Testing
// ---------------------------------------------------------------------------

// Test runs the Go test suite. Uses only cacheable flags so that Go's
// internal test result cache (GOCACHE) can skip unchanged packages
// across runs via the persistent go-build cache volume.
//
// +check
// +cache="session"
func (m *Go) Test(
	ctx context.Context,
	// Only run tests matching this regex.
	// +optional
	run string,
	// Skip tests matching this regex.
	// +optional
	skip string,
	// Abort test run on first failure.
	// +optional
	failfast bool,
	// How many tests to run in parallel. Defaults to the number of CPUs.
	// +optional
	// +default=0
	parallel int,
	// How long before timing out the test run.
	// +optional
	// +default="30m"
	timeout string,
	// Number of times to run each test. Zero uses Go's default (enables
	// test result caching).
	// +optional
	// +default=0
	count int,
	// Packages to test.
	// +optional
	// +default=["./..."]
	pkgs []string,
) error {
	if m.Race {
		m.Cgo = true
	}
	cmd := []string{"go", "test"}
	if parallel != 0 {
		cmd = append(cmd, fmt.Sprintf("-parallel=%d", parallel))
	}
	cmd = append(cmd, fmt.Sprintf("-timeout=%s", timeout))
	if count > 0 {
		cmd = append(cmd, fmt.Sprintf("-count=%d", count))
	}
	if run != "" {
		cmd = append(cmd, "-run", run)
	}
	if failfast {
		cmd = append(cmd, "-failfast")
	}
	if skip != "" {
		cmd = append(cmd, "-skip", skip)
	}
	_, err := m.Env("").
		WithExec(goCommand(cmd, pkgs, m.Ldflags, m.Values, m.Race)).
		Sync(ctx)
	return err
}

// TestCoverage runs Go tests with coverage profiling and returns the
// profile file. Runs independently of [Go.Test] because -coverprofile
// disables Go's internal test result caching. Dagger's layer caching
// still shares the base container layers (image, module download) with
// [Go.Test].
func (m *Go) TestCoverage() *dagger.File {
	return m.Env("").
		WithExec([]string{
			"go", "test", "-race", "-coverprofile=/tmp/coverage.txt", "./...",
		}).
		File("/tmp/coverage.txt")
}

// ---------------------------------------------------------------------------
// Linting
// ---------------------------------------------------------------------------

// Lint runs golangci-lint on the source code.
//
// +check
func (m *Go) Lint(ctx context.Context) error {
	_, err := m.lintBase().
		WithExec([]string{"golangci-lint", "run"}).
		Sync(ctx)
	return err
}

// LintPrettier checks YAML, JSON, and Markdown formatting.
//
// +check
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
//
// +check
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
	_, err := m.Env("").
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

// LintBase returns a golangci-lint container with source and caches. The
// Debian-based image is used (not Alpine) because it includes kernel headers
// needed by CGO transitive dependencies. The golangci-lint cache volume
// includes the linter version so that version bumps start fresh.
//
// Deprecated: Use [Go.Lint] instead. Exposed for backward compatibility
// with test modules.
func (m *Go) LintBase() *dagger.Container {
	return m.lintBase()
}
