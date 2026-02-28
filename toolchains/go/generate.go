package main

import "dagger/go/internal/dagger"

// FormatGo runs golangci-lint --fix and returns the changeset of
// Go source file changes against the original source directory.
func (m *Go) FormatGo() *dagger.Changeset {
	goFmt := m.lintBase().
		WithExec([]string{"golangci-lint", "run", "--fix"}).
		Directory("/src")
	return goFmt.Changes(m.Source)
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
