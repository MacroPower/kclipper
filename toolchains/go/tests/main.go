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

	"dagger/tests/internal/dagger"

	"golang.org/x/sync/errgroup"
)

// Tests provides integration tests for the [Go] module. Create instances
// with [New].
type Tests struct{}

// All runs all tests in parallel.
func (m *Tests) All(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return m.TestSourceFiltering(ctx) })
	g.Go(func() error { return m.TestGenerateIdempotent(ctx) })
	g.Go(func() error { return m.TestCoverageProfile(ctx) })
	g.Go(func() error { return m.TestEnv(ctx) })
	g.Go(func() error { return m.TestBuild(ctx) })
	g.Go(func() error { return m.TestBinary(ctx) })
	g.Go(func() error { return m.TestDownload(ctx) })
	g.Go(func() error { return m.TestModules(ctx) })
	g.Go(func() error { return m.TestModulesInclude(ctx) })
	g.Go(func() error { return m.TestModulesExclude(ctx) })
	g.Go(func() error { return m.TestLintModule(ctx) })
	g.Go(func() error { return m.TestTidyModule(ctx) })

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

// TestEnv verifies that [Go.Env] returns a working Go container with
// the expected environment configuration.
//
// +check
func (m *Tests) TestEnv(ctx context.Context) error {
	// Verify the container can run go version.
	out, err := dag.Go().Env(dagger.GoEnvOpts{}).
		WithExec([]string{"go", "version"}).
		Stdout(ctx)
	if err != nil {
		return fmt.Errorf("go version: %w", err)
	}
	if !strings.Contains(out, "go1.") {
		return fmt.Errorf("expected go version output, got: %s", out)
	}

	// Verify source is mounted.
	_, err = dag.Go().Env(dagger.GoEnvOpts{}).
		WithExec([]string{"test", "-f", "go.mod"}).
		Sync(ctx)
	if err != nil {
		return fmt.Errorf("go.mod not found in env: %w", err)
	}

	return nil
}

// TestBuild verifies that [Go.Build] compiles packages to the output directory.
//
// +check
func (m *Tests) TestBuild(ctx context.Context) error {
	dir := dag.Go(dagger.GoOpts{Cgo: true}).Build(dagger.GoBuildOpts{
		Pkgs:   []string{"./cmd/kclipper"},
		OutDir: "./bin/",
	})

	entries, err := dir.Entries(ctx)
	if err != nil {
		return fmt.Errorf("list entries: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("build produced no output")
	}

	// Verify there's a bin directory or binary.
	hasBin := false
	for _, entry := range entries {
		if strings.Contains(entry, "bin") {
			hasBin = true
			break
		}
	}
	if !hasBin {
		return fmt.Errorf("expected bin directory in output, got: %v", entries)
	}

	return nil
}

// TestBinary verifies that [Go.Binary] compiles a single package and
// returns the binary file.
//
// +check
func (m *Tests) TestBinary(ctx context.Context) error {
	binary := dag.Go(dagger.GoOpts{Cgo: true}).Binary("./cmd/kclipper", dagger.GoBinaryOpts{
		NoSymbols: true,
		NoDwarf:   true,
	})

	size, err := binary.Size(ctx)
	if err != nil {
		return fmt.Errorf("binary size: %w", err)
	}
	if size == 0 {
		return fmt.Errorf("binary has zero size")
	}

	return nil
}

// TestDownload verifies that [Go.Download] warms the module cache.
//
// +check
func (m *Tests) TestDownload(ctx context.Context) error {
	// Download returns a *Go; force evaluation by invoking Env and syncing.
	_, err := dag.Go().Download().Env(dagger.GoEnvOpts{}).Sync(ctx)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Multi-module tests
// ---------------------------------------------------------------------------

// TestModules verifies that [Go.Modules] discovers at least the root module.
//
// +check
func (m *Tests) TestModules(ctx context.Context) error {
	mods, err := dag.Go().Modules(ctx)
	if err != nil {
		return fmt.Errorf("modules: %w", err)
	}
	if len(mods) == 0 {
		return fmt.Errorf("expected at least one module, got none")
	}

	hasRoot := false
	for _, mod := range mods {
		if mod == "." {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		return fmt.Errorf("expected root module '.' in %v", mods)
	}
	return nil
}

// TestModulesInclude verifies that include patterns filter module results.
//
// +check
func (m *Tests) TestModulesInclude(ctx context.Context) error {
	mods, err := dag.Go().Modules(ctx, dagger.GoModulesOpts{
		Include: []string{"."},
	})
	if err != nil {
		return fmt.Errorf("modules with include: %w", err)
	}
	if len(mods) != 1 || mods[0] != "." {
		return fmt.Errorf("expected only root module '.', got %v", mods)
	}
	return nil
}

// TestModulesExclude verifies that exclude patterns filter out modules.
//
// +check
func (m *Tests) TestModulesExclude(ctx context.Context) error {
	mods, err := dag.Go().Modules(ctx, dagger.GoModulesOpts{
		Exclude: []string{"toolchains/**"},
	})
	if err != nil {
		return fmt.Errorf("modules with exclude: %w", err)
	}
	for _, mod := range mods {
		if strings.HasPrefix(mod, "toolchains/") {
			return fmt.Errorf("expected toolchains to be excluded, but found %q in %v", mod, mods)
		}
	}
	return nil
}

// TestLintModule verifies that [Go.LintModule] succeeds on the root module.
//
// +check
func (m *Tests) TestLintModule(ctx context.Context) error {
	err := dag.Go(dagger.GoOpts{Cgo: true}).LintModule(ctx, ".")
	if err != nil {
		return fmt.Errorf("lint module '.': %w", err)
	}
	return nil
}

// TestTidyModule verifies that [Go.TidyModule] returns an empty changeset
// on clean source.
//
// +check
func (m *Tests) TestTidyModule(ctx context.Context) error {
	changeset := dag.Go().TidyModule(".")

	empty, err := changeset.IsEmpty(ctx)
	if err != nil {
		return fmt.Errorf("check tidy changeset: %w", err)
	}
	if !empty {
		modified, _ := changeset.ModifiedPaths(ctx)
		return fmt.Errorf("expected empty changeset on clean source, modified=%v", modified)
	}
	return nil
}
