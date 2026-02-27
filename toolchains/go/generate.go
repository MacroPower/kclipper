package main

import "dagger/go/internal/dagger"

// Format runs golangci-lint --fix and prettier --write, returning the
// changeset against the original source directory.
//
// Both formatters operate on non-overlapping file types (.go vs
// .yaml/.md/.json), so they run against the original source in parallel.
// The results are merged by overlaying Prettier's output onto the
// Go-formatted source.
//
// +generate
func (m *Go) Format() *dagger.Changeset {
	patterns := defaultPrettierPatterns()

	// Go formatting via golangci-lint --fix.
	goFmt := m.lintBase().
		WithExec([]string{"golangci-lint", "run", "--fix"}).
		Directory("/src")

	// Prettier formatting (runs against original source in parallel with Go formatting).
	prettierFmt := m.PrettierBase().
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec(append(
			[]string{"prettier", "--config", "./.prettierrc.yaml", "-w"},
			patterns...,
		)).
		Directory("/src")

	// Merge: start with Go-formatted source, overlay Prettier-formatted files.
	// Dagger evaluates lazily, so both pipelines execute concurrently when the
	// changeset is resolved.
	formatted := goFmt.WithDirectory(".", prettierFmt, dagger.DirectoryWithDirectoryOpts{
		Include: patterns,
	})

	return formatted.Changes(m.Source)
}

// Generate runs go generate and returns the changeset of generated files
// against the original source.
//
// +generate
func (m *Go) Generate() *dagger.Changeset {
	generated := m.Env("").
		WithExec([]string{"go", "generate", "./..."}).
		Directory("/src").
		WithoutDirectory(".git")
	return generated.Changes(m.Source)
}
