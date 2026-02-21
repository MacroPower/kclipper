// Integration tests for the [Ci] module.

package main

import (
	"context"
	"fmt"
	"strings"
)

type Tests struct{}

// All runs all tests sequentially.
func (m *Tests) All(ctx context.Context) error {
	tests := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"TestSourceFiltering", m.TestSourceFiltering},
		{"TestFormatIdempotent", m.TestFormatIdempotent},
	}
	for _, tt := range tests {
		fmt.Printf("=== RUN   %s\n", tt.name)
		if err := tt.fn(ctx); err != nil {
			fmt.Printf("--- FAIL: %s\n", tt.name)
			return fmt.Errorf("%s: %w", tt.name, err)
		}
		fmt.Printf("--- PASS: %s\n", tt.name)
	}
	return nil
}

// TestSourceFiltering verifies that the +ignore annotation in [Ci.New]
// excludes the expected directories from the source.
func (m *Tests) TestSourceFiltering(ctx context.Context) error {
	entries, err := dag.Ci().Source().Entries(ctx)
	if err != nil {
		return fmt.Errorf("list source entries: %w", err)
	}

	excluded := []string{"dist", ".worktrees", ".tmp", ".devcontainer"}
	for _, dir := range excluded {
		for _, entry := range entries {
			if strings.TrimRight(entry, "/") == dir {
				return fmt.Errorf("source should exclude %q but it was present", dir)
			}
		}
	}

	// Verify essential files are present.
	required := []string{"go.mod", "ci"}
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
// source produces an empty changeset. This exercises the full [Ci.Format]
// pipeline (golangci-lint --fix + prettier --write) and confirms the source is
// clean.
func (m *Tests) TestFormatIdempotent(ctx context.Context) error {
	changeset := dag.Ci().Format()

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
