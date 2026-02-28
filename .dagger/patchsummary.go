package main

import (
	"fmt"
	"slices"
	"strings"
)

// summarizePatch parses a unified diff and returns a human-readable summary
// showing each changed file with its added/removed line counts.
func summarizePatch(patch string) string {
	type entry struct {
		name    string
		mode    string
		added   int
		removed int
	}

	var entries []entry
	for _, section := range strings.Split("\n"+patch, "\ndiff --git ")[1:] {
		lines := strings.SplitN(section, "\n", -1)
		if len(lines) == 0 {
			continue
		}

		// Extract filename from "a/path b/path".
		parts := strings.SplitN(lines[0], " ", 2)
		name := strings.TrimPrefix(parts[len(parts)-1], "b/")

		mode := "Modified"
		var added, removed int
		for _, line := range lines[1:] {
			switch {
			case strings.HasPrefix(line, "new file"):
				mode = "Added"
			case strings.HasPrefix(line, "deleted file"):
				mode = "Deleted"
			case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
				added++
			case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
				removed++
			}
		}
		entries = append(entries, entry{name, mode, added, removed})
	}

	if len(entries) == 0 {
		return patch
	}

	slices.SortFunc(entries, func(a, b entry) int {
		return strings.Compare(a.name, b.name)
	})

	var b strings.Builder
	totalAdded, totalRemoved := 0, 0
	for _, e := range entries {
		fmt.Fprintf(&b, "%s: %s  (+%d -%d)\n", e.mode, e.name, e.added, e.removed)
		totalAdded += e.added
		totalRemoved += e.removed
	}
	fmt.Fprintf(&b, "\n%d files changed, +%d -%d lines", len(entries), totalAdded, totalRemoved)
	return b.String()
}
