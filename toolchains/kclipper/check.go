package main

import (
	"context"

	"dagger/kclipper/internal/dagger"
)

// LintReleaser validates the GoReleaser configuration. Uses
// [Kclipper.goreleaserCheckBase] with the kclipper remote URL because the
// goreleaser config references a git remote for homebrew/nix repository
// resolution.
//
// +check
func (m *Kclipper) LintReleaser(ctx context.Context) error {
	ctr, err := m.goreleaserCheckBase(ctx, kclipperCloneURL)
	if err != nil {
		return err
	}
	_, err = ctr.
		WithExec([]string{"goreleaser", "check"}).
		Sync(ctx)
	return err
}

// LintPrettier checks YAML, JSON, and Markdown formatting.
//
// +check
func (m *Kclipper) LintPrettier(
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
	_, err := m.prettierBase().
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec(args).
		Sync(ctx)
	return err
}

// LintActions runs zizmor to lint GitHub Actions workflows.
//
// +check
func (m *Kclipper) LintActions(ctx context.Context) error {
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

// LintDeadcode reports unreachable functions in the codebase using the
// golang.org/x/tools deadcode analyzer. This is an advisory lint that
// is not included in standard checks; invoke via dagger call kclipper lint-deadcode.
func (m *Kclipper) LintDeadcode(ctx context.Context) error {
	_, err := m.Go.Env(dagger.GoEnvOpts{}).
		WithExec([]string{
			"go", "install",
			"golang.org/x/tools/cmd/deadcode@" + deadcodeVersion,
		}).
		WithExec([]string{"deadcode", "./..."}).
		Sync(ctx)
	return err
}
