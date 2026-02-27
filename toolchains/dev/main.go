// Reusable development container functions for Go projects. Provides
// pre-configured dev environments with shell tools, Claude Code, and
// git-based source management.

package main

import (
	"strings"
	"time"

	"dagger/dev/internal/dagger"
)

const (
	goVersion         = "1.25"            // renovate: datasource=golang-version depName=go
	taskVersion       = "v3.48.0"         // renovate: datasource=github-releases depName=go-task/task
	lefthookVersion   = "v2.1.1"          // renovate: datasource=github-releases depName=evilmartians/lefthook
	daggerVersion     = "v0.19.11"        // renovate: datasource=github-releases depName=dagger/dagger
	starshipVersion   = "v1.24.2"         // renovate: datasource=github-releases depName=starship/starship
	yqVersion         = "v4.52.4"         // renovate: datasource=github-releases depName=mikefarah/yq
	uvVersion         = "0.10.4"          // renovate: datasource=github-releases depName=astral-sh/uv extractVersion=^(?P<version>.*)$
	ghVersion         = "v2.87.2"         // renovate: datasource=github-releases depName=cli/cli
	claudeCodeVersion = "2.1.50"          // renovate: datasource=npm depName=@anthropic-ai/claude-code
)

// starshipConfig is the starship prompt configuration written to
// ~/.config/starship.toml inside the dev container.
const starshipConfig = `add_newline = false
palette = 'one_dark'
format = "$directory$git_branch$git_status$golang$fill$cmd_duration$line_break$character"

[fill]
symbol = ' '

[directory]
truncation_length = 3
style = 'bold blue'

[git_branch]
format = '[$symbol$branch]($style) '
symbol = '@ '
style = 'bold purple'

[git_status]
format = '([$all_status$ahead_behind]($style) )'
style = 'bold yellow'

[golang]
format = '[$symbol$version]($style) '
symbol = 'go '
style = 'bold cyan'

[cmd_duration]
min_time = 2_000
format = '[$duration]($style)'
style = 'comment'

[character]
success_symbol = '[>](bold green)'
error_symbol = '[>](bold red)'

[palettes.one_dark]
red = '#E06C75'
green = '#98C379'
yellow = '#E5C07B'
blue = '#61AFEF'
purple = '#C678DD'
cyan = '#56B6C2'
white = '#ABB2BF'
comment = '#5C6370'
`

// zshConfig is the zsh configuration written to ~/.zshrc inside the dev
// container.
const zshConfig = `# History (persisted via cache volume)
HISTFILE=/commandhistory/.zsh_history
HISTSIZE=10000
SAVEHIST=10000
setopt HIST_IGNORE_ALL_DUPS SHARE_HISTORY APPEND_HISTORY INC_APPEND_HISTORY

# Completions
autoload -Uz compinit && compinit
zstyle ':completion:*' menu select
zstyle ':completion:*' matcher-list 'm:{a-z}={A-Z}'
zstyle ':completion:*' list-colors "${(s.:.)LS_COLORS}"

# Plugins
source /usr/share/zsh-autosuggestions/zsh-autosuggestions.zsh
ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE='fg=244'
source /usr/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh

# Colors
eval "$(dircolors -b)"

# fzf integration
source /usr/share/doc/fzf/examples/key-bindings.zsh
source /usr/share/doc/fzf/examples/completion.zsh
export FZF_DEFAULT_COMMAND='fd --type f --hidden --follow --exclude .git'
export FZF_DEFAULT_OPTS='--height=40% --layout=reverse --border --color=fg:-1,bg:-1,hl:cyan,fg+:white,bg+:236,hl+:cyan,info:yellow,prompt:green,pointer:magenta,marker:magenta'

# Tool config
export BAT_THEME='ansi'

# Aliases
alias ls='ls --color=auto'
alias ll='ls -lh'
alias la='ls -lAh'
alias l='ls -CF'
alias grep='grep --color=auto'
alias cat='bat --paging=never'

# direnv
eval "$(direnv hook zsh)"

# Starship prompt (must be last)
eval "$(starship init zsh)"
`

// devInitScript is the shell script that initializes the git repository
// and overlays local source files in the dev container. It expects BRANCH,
// BASE, and CLONE_URL environment variables to be set.
const devInitScript = `set -e

# Clone if needed (blobless: full history, blobs fetched on demand).
if [ ! -d /src/.git ]; then
  git clone --filter=blob:none --no-checkout \
    "${CLONE_URL}" /src
fi

cd /src

# Fetch latest refs from origin. Non-fatal when the branch already
# exists locally (cached in the Dagger volume from a prior session).
if ! git fetch origin; then
  if git rev-parse --verify "${BRANCH}" >/dev/null 2>&1; then
    echo "WARNING: git fetch origin failed, using cached branch '${BRANCH}'" >&2
  else
    echo "ERROR: git fetch origin failed and branch '${BRANCH}' has no local cache" >&2
    exit 1
  fi
fi

# Checkout or create the branch. Force checkout (-f) avoids "untracked
# working tree files would be overwritten" errors when the cache volume
# retains files from a previous session that are now tracked on the branch.
if git rev-parse --verify "${BRANCH}" >/dev/null 2>&1; then
  git checkout -f "${BRANCH}"
  # Advance local branch to match remote. The cache volume may hold a
  # stale branch tip from a previous session; git fetch updated
  # origin/${BRANCH} but the local ref wasn't moved. Any prior-session
  # commits were already exported to the host by _dev-sync, and the
  # working tree is about to be replaced by rsync, so reset is safe.
  if git rev-parse --verify "origin/${BRANCH}" >/dev/null 2>&1; then
    git reset --hard "origin/${BRANCH}"
  fi
elif git rev-parse --verify "origin/${BRANCH}" >/dev/null 2>&1; then
  git checkout -f -b "${BRANCH}" "origin/${BRANCH}"
elif git rev-parse --verify "origin/${BASE}" >/dev/null 2>&1; then
  git checkout -f -b "${BRANCH}" "origin/${BASE}"
else
  echo "ERROR: cannot create branch '${BRANCH}': ref 'origin/${BASE}' does not exist" >&2
  echo "Ensure the base branch '${BASE}' exists on the remote." >&2
  exit 1
fi

# Validate seed before overlay to prevent wiping /src with empty source.
if [ ! -f /tmp/src-seed/go.mod ]; then
  echo "ERROR: seed validation failed: /tmp/src-seed/go.mod not found" >&2
  exit 1
fi

# Overlay local source (m.Source excludes .git via +ignore).
# rsync --delete removes files present in git but deleted locally.
rsync -a --delete --exclude=.git /tmp/src-seed/ /src/
`

// Dev provides reusable development container functions for Go projects.
// Create instances with [New].
type Dev struct {
	// Project source directory.
	Source *dagger.Directory
}

// New creates a [Dev] module with the given project source directory.
func New(
	// Project source directory.
	// +defaultPath="/"
	// +ignore=["dist", ".worktrees", ".tmp", ".git"]
	source *dagger.Directory,
) *Dev {
	return &Dev{Source: source}
}

// DevBase returns a base development container with Go, shell tools,
// and Claude Code pre-installed but no source mounted. Used by integration
// tests to verify tool availability without requiring an interactive
// terminal.
func (m *Dev) DevBase() *dagger.Container {
	return dag.Container().
		From("golang:"+goVersion).
		// Mount apt cache volumes so re-runs skip network downloads.
		WithMountedCache("/var/cache/apt/archives", dag.CacheVolume("dev-apt-archives")).
		WithMountedCache("/var/lib/apt/lists", dag.CacheVolume("dev-apt-lists")).
		WithExec([]string{"sh", "-c",
			"apt-get update && apt-get install -y --no-install-recommends " +
				"curl less man-db gnupg2 nano vim xz-utils jq wget dnsutils direnv " +
				"zsh zsh-autosuggestions zsh-syntax-highlighting " +
				"ripgrep fd-find bat fzf tree htop rsync " +
				"nodejs npm",
		}).
		// Symlink Debian-renamed binaries to their canonical names.
		WithExec([]string{"sh", "-c",
			"ln -s /usr/bin/batcat /usr/local/bin/bat && " +
				"ln -s /usr/bin/fdfind /usr/local/bin/fd",
		}).
		// All tool binaries from a single builder sub-pipeline.
		WithDirectory("/usr/local/bin", devToolBins()).
		WithDirectory("/root/.local", claudeCodeFiles()).
		// Shell config.
		WithNewFile("/root/.config/starship.toml", starshipConfig).
		WithNewFile("/root/.zshrc", zshConfig).
		// Editor and terminal env vars.
		WithEnvVariable("EDITOR", "nano").
		WithEnvVariable("VISUAL", "nano").
		WithEnvVariable("TERM", "xterm-256color").
		// Signal that this environment is sandboxed (isolated Dagger
		// container). Without this, Claude Code refuses to run with
		// --dangerously-skip-permissions when the user is root.
		WithEnvVariable("IS_SANDBOX", "1").
		WithEnvVariable("PATH", "/root/.local/bin:$PATH",
			dagger.ContainerWithEnvVariableOpts{Expand: true})
}

// DevEnv returns a development container with the git repository cloned,
// the requested branch checked out, and local source files overlaid.
// Cache volumes provide per-branch workspace isolation and shared Go
// module/build caches. Unlike [Dev.Dev], this does not open an interactive
// terminal or export results.
func (m *Dev) DevEnv(
	// Branch to check out in the dev container. Each branch gets its
	// own Dagger cache volume for workspace isolation.
	branch string,
	// Git clone URL for the repository.
	cloneURL string,
	// Base branch name used when creating a new branch that does not
	// exist locally or on the remote. Looked up as origin/<base> in
	// the container clone. Defaults to "main" when empty.
	// +optional
	base string,
	// Override the base container. Uses [Dev.DevBase] when nil.
	// +optional
	ctr *dagger.Container,
) *dagger.Container {
	if base == "" {
		base = "main"
	}
	if ctr == nil {
		ctr = m.DevBase()
	}

	return ctr.
		// Stage source on regular filesystem for the seed step.
		WithDirectory("/tmp/src-seed", m.Source).
		// Cache volume at /src so changes survive Terminal().
		// Each branch gets its own volume for workspace isolation.
		WithMountedCache("/src", dag.CacheVolume("dev-src-"+sanitizeCacheKey(branch))).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod-"+goVersion)).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build-"+goVersion)).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithMountedCache("/commandhistory", dag.CacheVolume("shell-history")).
		WithWorkdir("/src").
		WithEnvVariable("BRANCH", branch).
		WithEnvVariable("BASE", base).
		WithEnvVariable("CLONE_URL", cloneURL).
		// _DEV_TS busts the Dagger function cache on every call. Without
		// it, if m.Source hasn't changed, Dagger returns a cached DevEnv()
		// result and skips git fetch origin, so remote branch updates
		// would not be picked up.
		WithEnvVariable("_DEV_TS", time.Now().String()).
		WithExec([]string{"sh", "-c", devInitScript})
}

// Dev opens an interactive development container with a real git
// repository and returns the modified source directory when the session
// ends. The container is created via [Dev.DevEnv], which clones the
// upstream repo (blobless) and checks out the specified branch, enabling
// pushes, rebases, and other git operations.
//
// Source files from the project directory are overlaid on top of the
// checked-out branch, bringing in local uncommitted changes. Each branch
// gets its own Dagger cache volume for workspace isolation.
//
// The returned directory includes .git with full commit history.
//
// +cache="never"
func (m *Dev) Dev(
	// Branch to check out in the dev container. Each branch gets its
	// own Dagger cache volume for workspace isolation.
	branch string,
	// Git clone URL for the repository.
	cloneURL string,
	// Base branch name used when creating a new branch that does not
	// exist locally or on the remote. Looked up as origin/<base> in
	// the container clone. Defaults to "main" when empty.
	// +optional
	base string,
	// Override the base container. Uses [Dev.DevBase] when nil.
	// +optional
	ctr *dagger.Container,
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
	devCtr := m.DevEnv(branch, cloneURL, base, ctr)

	devCtr = applyDevConfig(devCtr, claudeConfig, claudeJSON, gitConfig, ccstatuslineConfig,
		tz, colorterm, termProgram, termProgramVersion)

	// Pre-download Go modules (non-fatal: user can fix go.mod interactively).
	devCtr = devCtr.WithExec([]string{"sh", "-c",
		"go mod download || echo 'WARNING: go mod download failed; run it manually' >&2",
	})

	// Open interactive terminal. Changes to /src persist in the cache
	// volume through the Terminal() call.
	if len(cmd) == 0 {
		cmd = []string{"zsh"}
	}
	devCtr = devCtr.Terminal(dagger.ContainerTerminalOpts{
		Cmd:                           cmd,
		ExperimentalPrivilegedNesting: true,
	})

	// Copy from cache volume to regular filesystem so Directory() can
	// read it (Container.Directory rejects cache mount paths).
	devCtr = devCtr.WithExec([]string{"sh", "-c", "mkdir -p /output && cp -a /src/. /output/"})

	return devCtr.Directory("/output")
}

// applyDevConfig applies optional configuration mounts and environment
// variables to a dev container.
func applyDevConfig(
	ctr *dagger.Container,
	claudeConfig *dagger.Directory,
	claudeJSON *dagger.File,
	gitConfig *dagger.Directory,
	ccstatuslineConfig *dagger.Directory,
	tz, colorterm, termProgram, termProgramVersion string,
) *dagger.Container {
	if claudeConfig != nil {
		ctr = ctr.
			WithMountedDirectory("/tmp/claude-config-seed", claudeConfig).
			WithMountedCache("/root/.claude", dag.CacheVolume("claude-config")).
			WithExec([]string{"rsync", "-a", "/tmp/claude-config-seed/", "/root/.claude/"})
	}
	if claudeJSON != nil {
		ctr = ctr.WithMountedFile("/root/.claude.json", claudeJSON)
	}
	if gitConfig != nil {
		ctr = ctr.WithMountedDirectory("/root/.config/git", gitConfig)
	}
	if ccstatuslineConfig != nil {
		ctr = ctr.WithMountedDirectory("/root/.config/ccstatusline", ccstatuslineConfig)
	}
	if tz != "" {
		ctr = ctr.WithEnvVariable("TZ", tz)
	}
	if colorterm != "" {
		ctr = ctr.WithEnvVariable("COLORTERM", colorterm)
	}
	if termProgram != "" {
		ctr = ctr.WithEnvVariable("TERM_PROGRAM", termProgram)
	}
	if termProgramVersion != "" {
		ctr = ctr.WithEnvVariable("TERM_PROGRAM_VERSION", termProgramVersion)
	}
	return ctr
}

// sanitizeCacheKey replaces characters that are invalid in Dagger cache
// volume names with hyphens.
func sanitizeCacheKey(name string) string {
	return strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(name)
}

// ---------------------------------------------------------------------------
// Dev container helpers (private)
// ---------------------------------------------------------------------------

// devToolBins returns a directory containing all dev tool binaries.
// Everything is built in a single alpine container so Dagger resolves
// one sub-pipeline for all tools. GitHub release downloads run in one
// exec; OCI image binaries are added via [dagger.Container.WithFile].
func devToolBins() *dagger.Directory {
	ghVer := strings.TrimPrefix(ghVersion, "v")
	lefthookVer := strings.TrimPrefix(lefthookVersion, "v")

	// Reuse a single container for uv and uvx (same image).
	uvCtr := dag.Container().From("ghcr.io/astral-sh/uv:" + uvVersion)

	return dag.Container().
		From("alpine:3").
		WithExec([]string{"mkdir", "-p", "/tools"}).
		// Download all GitHub release tools in one exec.
		WithExec([]string{"sh", "-c",
			"ARCH=$(uname -m) && " +
				"GOARCH=$(echo $ARCH | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') && " +
				// starship
				"wget -qO- https://github.com/starship/starship/releases/download/" + starshipVersion +
				"/starship-${ARCH}-unknown-linux-musl.tar.gz | tar xz -C /tools && " +
				// task
				"wget -qO- https://github.com/go-task/task/releases/download/" + taskVersion +
				"/task_linux_${GOARCH}.tar.gz | tar xz -C /tools task && " +
				// lefthook
				"wget -qO /tools/lefthook https://github.com/evilmartians/lefthook/releases/download/" + lefthookVersion +
				"/lefthook_" + lefthookVer + "_Linux_${ARCH} && chmod +x /tools/lefthook && " +
				// gh
				"wget -qO- https://github.com/cli/cli/releases/download/" + ghVersion +
				"/gh_" + ghVer + "_linux_${GOARCH}.tar.gz | " +
				"tar xz -O gh_" + ghVer + "_linux_${GOARCH}/bin/gh > /tools/gh && chmod +x /tools/gh",
		}).
		// OCI image binaries.
		WithFile("/tools/dagger",
			dag.Container().From("registry.dagger.io/engine:"+daggerVersion).
				File("/usr/local/bin/dagger")).
		WithFile("/tools/yq",
			dag.Container().From("mikefarah/yq:"+strings.TrimPrefix(yqVersion, "v")).
				File("/usr/bin/yq")).
		WithFile("/tools/uv", uvCtr.File("/uv")).
		WithFile("/tools/uvx", uvCtr.File("/uvx")).
		Directory("/tools")
}

// claudeCodeFiles returns the Claude Code installation directory from a
// pinned install script run inside a debian-slim builder.
func claudeCodeFiles() *dagger.Directory {
	return dag.Container().
		From("debian:13-slim").
		WithExec([]string{"sh", "-c",
			"apt-get update && apt-get install -y --no-install-recommends curl ca-certificates",
		}).
		WithExec([]string{"sh", "-c",
			"curl -fsSL https://claude.ai/install.sh | bash -s -- " + claudeCodeVersion,
		}).
		Directory("/root/.local")
}
