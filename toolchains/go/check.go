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
