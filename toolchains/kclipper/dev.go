package main

import "dagger/kclipper/internal/dagger"

// DevEnv returns a development container with the git repository cloned,
// the requested branch checked out, and local source files overlaid.
// Cache volumes provide per-branch workspace isolation and shared Go
// module/build caches. Unlike [Kclipper.Dev], this does not open an interactive
// terminal or export results.
func (m *Kclipper) DevEnv(
	// Branch to check out in the dev container. Each branch gets its
	// own Dagger cache volume for workspace isolation.
	branch string,
	// Base branch name used when creating a new branch that does not
	// exist locally or on the remote. Looked up as origin/<base> in
	// the container clone. Defaults to "main" when empty.
	// +optional
	base string,
) *dagger.Container {
	return m.devToolchain().DevEnv(branch, kclipperCloneURL, dagger.DevDevEnvOpts{
		Base: base,
	})
}

// Dev opens an interactive development container with a real git
// repository and returns the modified source directory when the session
// ends. The container is created via [Kclipper.DevEnv], which clones the
// upstream repo (blobless) and checks out the specified branch, enabling
// pushes, rebases, and other git operations.
//
// Source files from the project directory are overlaid on top of the
// checked-out branch, bringing in local uncommitted changes. Each branch
// gets its own Dagger cache volume for workspace isolation.
//
// The returned directory includes .git with full commit history. Use the
// Taskfile dev/claude tasks to handle the translation between the
// container's standalone .git and the host's worktree format.
//
// Usage:
//
//	task dev                        # defaults to current branch
//	task dev BRANCH=feat/my-work    # explicit branch, base = current branch
//	task claude BRANCH=feat/my-work BASE=main  # explicit base
//
// +cache="never"
func (m *Kclipper) Dev(
	// Branch to check out in the dev container. Each branch gets its
	// own Dagger cache volume for workspace isolation.
	branch string,
	// Base branch name used when creating a new branch that does not
	// exist locally or on the remote. Looked up as origin/<base> in
	// the container clone. Defaults to "main" when empty.
	// +optional
	base string,
	// Claude Code configuration directory (~/.claude).
	// +optional
	// +ignore=["debug", "projects", "todos", "file-history", "plans", "tasks", "teams", "session-env", "backups", "paste-cache", "cache", "telemetry", "downloads", "shell-snapshots", "history.jsonl", ".claude.json*", "stats-cache.json", "statsig", "skills"]
	claudeConfig *dagger.Directory,
	// Claude Code settings file (~/.claude.json).
	// +optional
	claudeJSON *dagger.File,
	// Git configuration directory (~/.config/git).
	// +optional
	gitConfig *dagger.Directory,
	// Claude Code status line configuration directory (~/.config/ccstatusline).
	// +optional
	ccstatuslineConfig *dagger.Directory,
	// Timezone for the container (e.g. "America/New_York").
	// +optional
	tz string,
	// COLORTERM value (e.g. "truecolor").
	// +optional
	colorterm string,
	// TERM_PROGRAM value (e.g. "Apple_Terminal", "iTerm.app").
	// +optional
	termProgram string,
	// TERM_PROGRAM_VERSION value.
	// +optional
	termProgramVersion string,
	// Command to run in the terminal session. Defaults to ["zsh"].
	// +optional
	cmd []string,
) *dagger.Directory {
	return m.devToolchain().Dev(branch, kclipperCloneURL, dagger.DevDevOpts{
		Base:               base,
		ClaudeConfig:       claudeConfig,
		ClaudeJSON:         claudeJSON,
		GitConfig:          gitConfig,
		CcstatuslineConfig: ccstatuslineConfig,
		Tz:                 tz,
		Colorterm:          colorterm,
		TermProgram:        termProgram,
		TermProgramVersion: termProgramVersion,
		Cmd:                cmd,
	})
}
