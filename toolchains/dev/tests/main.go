// Integration tests for the [Dev] module. Individual tests are annotated
// with +check so `dagger check -m toolchains/dev/tests` runs them all concurrently.
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

const (
	// Clone URL used by dev container tests.
	testCloneURL = "https://github.com/macropower/kclipper.git"
)

// Tests provides integration tests for the [Dev] module. Create instances
// with [New].
type Tests struct{}

// All runs all tests in parallel.
func (m *Tests) All(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return m.TestDevBase(ctx) })
	g.Go(func() error { return m.TestDevExportPersistence(ctx) })
	g.Go(func() error { return m.TestDevEnvDefaultBase(ctx) })

	return g.Wait()
}

// TestDevBase verifies that [Dev.DevBase] produces a container with essential
// development tools available on PATH. This validates the tool installation
// pipeline without requiring an interactive terminal session.
//
// +check
func (m *Tests) TestDevBase(ctx context.Context) error {
	ctr := dag.Dev().DevBase()

	tools := []string{
		"go", "task", "dagger", "lefthook", "claude",
		"starship", "yq", "uv", "gh", "direnv",
		"rg", "fd", "bat", "fzf", "tree", "htop",
		"node", "npm", "npx",
	}
	for _, tool := range tools {
		_, err := ctr.WithExec([]string{"which", tool}).Sync(ctx)
		if err != nil {
			return fmt.Errorf("%s not found in dev container: %w", tool, err)
		}
	}

	return nil
}

// TestDevExportPersistence verifies that the [Dev.Dev] export pipeline
// correctly captures new files, preserves original source files, and
// includes the .git directory with real commit history. This uses
// [Dev.DevEnv] to set up the same environment as [Dev.Dev] but replaces
// [dagger.Container.Terminal] with [dagger.Container.WithExec] to
// simulate interactive changes.
//
// +check
func (m *Tests) TestDevExportPersistence(ctx context.Context) error {
	branch := "test-dev-export"

	exported := dag.Dev().DevEnv(branch, testCloneURL, dagger.DevDevEnvOpts{Base: "main"}).
		// Simulate interactive changes: create a new file + modify an existing one.
		WithExec([]string{"sh", "-c",
			"echo test-content > /src/test-sentinel && " +
				"echo '# modified' >> /src/README.md",
		}).
		// Copy to output (same as Dev pipeline).
		WithExec([]string{"sh", "-c", "mkdir -p /output && cp -a /src/. /output/"}).
		Directory("/output")

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

	// Verify .git is present in export (real git repo).
	if !foundGit {
		return fmt.Errorf(".git should be present in export but was missing (entries: %v)", entries)
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

// TestDevEnvDefaultBase verifies that [Dev.DevEnv] defaults the BASE
// environment variable to "main" when no base is provided.
//
// +check
func (m *Tests) TestDevEnvDefaultBase(ctx context.Context) error {
	base, err := dag.Dev().DevEnv("test-default-base", testCloneURL).EnvVariable(ctx, "BASE")
	if err != nil {
		return fmt.Errorf("read BASE env var: %w", err)
	}
	if base != "main" {
		return fmt.Errorf("BASE = %q, want %q", base, "main")
	}
	return nil
}
