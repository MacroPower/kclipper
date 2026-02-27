// Commitlint validates commit messages against a project's conventional
// commit policy using commitlint.

package main

import (
	"context"

	"dagger/commitlint/internal/dagger"
)

const (
	defaultImage = "commitlint/commitlint:19.9.1" // renovate: datasource=docker depName=commitlint/commitlint
)

// Commitlint validates commit messages against a project's conventional
// commit policy. Create instances with [New].
type Commitlint struct {
	// Container image with commitlint installed.
	Image string
}

// New creates a new [Commitlint] module.
func New(
	// Container image with commitlint installed.
	// +optional
	image string,
) *Commitlint {
	if image == "" {
		image = defaultImage
	}
	return &Commitlint{Image: image}
}

// Lint validates commit messages against the project's commitlint
// configuration.
func (m *Commitlint) Lint(
	ctx context.Context,
	// Project source directory containing the commitlint config.
	source *dagger.Directory,
	// Commit message file to validate (e.g. .git/COMMIT_EDITMSG).
	// +optional
	msgFile *dagger.File,
	// Arguments to pass to commitlint.
	// +optional
	args []string,
) error {
	ctr := dag.Container().
		From(m.Image).
		WithMountedDirectory("/src", source).
		WithWorkdir("/src")

	if msgFile != nil {
		ctr = ctr.WithMountedFile("/tmp/commit-msg", msgFile)
		args = append(args, "--edit", "/tmp/commit-msg")
	}

	_, err := ctr.
		WithExec(args, dagger.ContainerWithExecOpts{UseEntrypoint: true}).
		Sync(ctx)
	return err
}
