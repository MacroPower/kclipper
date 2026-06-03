package main

import "dagger/kclipper/internal/dagger"

// Format runs golangci-lint --fix and prettier --write, returning the
// changeset against the original source directory.
//
// Both formatters operate on non-overlapping file types (.go vs
// .yaml/.md/.json), so they run against the original source in parallel.
// The changesets are merged using [dagger.Changeset.WithChangeset].
//
// +generate
func (m *Kclipper) Format() *dagger.Changeset {
	// Go formatting via golangci-lint --fix (delegated to Go module).
	goChangeset := m.Go.FormatGo()

	// Prettier formatting (delegated to the prettier module, runs against the
	// original source in parallel with Go formatting).
	prettierChangeset := m.Prettier.Format()

	// Merge: both changesets operate on non-overlapping file types (.go vs
	// .yaml/.md/.json), so Dagger evaluates them concurrently with no conflicts.
	return goChangeset.WithChangeset(prettierChangeset)
}
