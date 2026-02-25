# CI Module

Dagger-based CI/CD module for kclipper. All CI tasks (testing, linting,
formatting, building, releasing) run in containers orchestrated by Dagger.

## Quick Reference

```bash
dagger check                  # Run all checks (+check functions)
dagger check lint             # Run specific check(s)
dagger call lint-deadcode     # Run deadcode analysis (opt-in, advisory)
dagger generate --auto-apply  # Run generators (Format + Generate) and apply changes
dagger call build --output=./dist       # Build binaries
task dev                      # Dev container (branch defaults to current)
task dev BRANCH=feat/foo      # Dev container with explicit branch
task claude                   # Claude Code container (branch defaults to current)
task claude BRANCH=feat/foo   # Claude Code container with explicit branch
dagger call lint-commit-msg --msg-file=.git/COMMIT_EDITMSG  # Validate commit message
```

## Architecture

The module is a single Go package (`ci/main.go`) exposing public methods as
Dagger Functions. The `dagger.json` at the repo root configures the module:

```
dagger.json          # Module config (name, engine version, SDK, source dir)
ci/
  main.go            # All CI functions
  go.mod             # Module dependencies (dagger SDK, otel, etc.)
  dagger.gen.go      # Auto-generated (do not edit)
  internal/dagger/   # Auto-generated SDK (do not edit)
  tests/             # Test module (imports ci as dependency)
```

Go build environments are created manually using `golang:` base images (see
`goBase()`, `lintBase()`, `releaserBase()`).

`ci/tests/` is a separate Dagger module that imports CI and tests its public
API. To add a test: add a `+check`-annotated method on `Tests` and register it
in `All`.

### Function Categories

| Category    | Annotation     | CLI                          | Purpose                                        |
| ----------- | -------------- | ---------------------------- | ---------------------------------------------- |
| Checks      | `// +check`    | `dagger check <name>`        | Validation (tests, lints). Return `error`.     |
| Generators  | `// +generate` | `dagger generate`            | Code formatting. Return `*dagger.Changeset`.   |
| Build       | (none)         | `dagger call <name>`         | Artifact production.                           |
| Callable    | (none)         | `dagger call <name>`         | Requires arguments; invoked via `dagger call`. |
| Development | (none)         | `dagger call dev --output=.` | Interactive container with persistent export.  |
| Testable    | (none)         | `dagger call <name>`         | Non-interactive building blocks for tests.     |

`LintCommitMsg` is in the Callable category because it requires a mandatory
`msgFile` argument and cannot be a `+check` function.

`LintDeadcode` is in the Callable category as an opt-in advisory lint. It is
not a `+check` function so it does not run during `dagger check`.

`DevEnv` and `DevBase` are in the Testable category: they expose intermediate
pipeline stages that the test module exercises without needing `Terminal()`.

## Adding a New Check

1. Add a public method with `// +check` annotation:

```go
// LintFoo checks foo.
//
// +check
func (m *Ci) LintFoo(ctx context.Context) error {
    _, err := dag.Container().
        From("image:tag").
        WithMountedDirectory("/src", m.Source).
        WithWorkdir("/src").
        WithExec([]string{"foo", "check"}).
        Sync(ctx)
    return err
}
```

2. Add to the lint task in `Taskfile.yaml`.
3. Add to `.github/workflows/validate.yaml` if it should run in CI.

## Adding a New Generator

1. Add a public method with `// +generate` annotation that returns `*dagger.Changeset`:

```go
// GenerateFoo generates foo files.
//
// +generate
func (m *Ci) GenerateFoo() *dagger.Changeset {
    generated := dag.Container().
        From("image:tag").
        WithMountedDirectory("/src", m.Source).
        WithWorkdir("/src").
        WithExec([]string{"foo", "generate"}).
        Directory("/src")
    return generated.Changes(m.Source)
}
```

2. Run with `dagger generate --auto-apply` to apply changes locally.

## Conventions

- All public function parameters must have doc comments (they become
  `dagger call --help` text).
- Use Go doc link syntax (`[Name]`, `[*Name]`) in doc comments per the
  project's root `CLAUDE.md`.
- The package-level doc comment in `main.go` provides the module description
  shown in `dagger functions` and `dagger call --help`.
- Functions with external side effects (publishing, releasing) must use
  `// +cache="never"` to prevent stale cached results. All other functions
  use Dagger's default function caching.
- Pure-logic utilities are public methods directly (no private helper
  indirection) so they remain testable from the test module.
- Registry auth (`WithRegistryAuth`) is conditional: only applied when
  `registryPassword` is non-nil, allowing anonymous registries for testing.
- Use `env://VAR_NAME` syntax for Dagger secret providers on the CLI.
- Use `errgroup` for concurrent execution of independent operations.
- Tool binaries are extracted from OCI images via `Container.File()`
  (not `go install`) for faster builds and automatic platform matching.

## Dev Container

The `Dev()` function creates a git-aware development container with real commit
history. It delegates to `DevEnv()` for environment setup (git clone, branch
checkout, source overlay), then adds configuration mounts, `go mod download`,
the interactive terminal, and export. The `devInitScript` constant holds the
shared shell script for git initialization.

Key behaviors:

- **Blobless clone**: The upstream repo is cloned with `--filter=blob:none`,
  providing full commit history while fetching file contents on demand.
- **Branch isolation**: Each branch gets its own Dagger cache volume
  (`dev-src-<branch>`), so switching branches doesn't clobber in-progress work.
- **Base branch**: The optional `base` parameter (defaults to `"main"`) controls
  which remote branch is used as the starting point when creating a new branch
  that doesn't exist locally or on the remote (`origin/<base>`). The Taskfile
  tasks default `BASE` to the current host branch, so `task dev BRANCH=feat/new`
  inherits from where you are now rather than always branching from `main`.
- **Non-fatal git fetch**: `git fetch origin` in `devInitScript` is non-fatal
  when the branch already exists locally (cached in the Dagger volume from a
  prior session). This allows offline use or recovery when the remote is
  temporarily unreachable. The fetch is only fatal when the branch has never
  been checked out before and has no local cache.
- **Force checkout**: All three checkout paths use `git checkout -f` (including
  `-f -b` for branch creation) to avoid "untracked working tree files would be
  overwritten" errors. The cache volume can retain files from a previous session
  that conflict with the target branch after a fetch. Since `rsync --delete`
  overlays the host's source files immediately after checkout, the working tree
  state at checkout time doesn't matter.
- **Branch reset to remote**: After checking out an existing local branch,
  `devInitScript` runs `git reset --hard origin/${BRANCH}` to advance the local
  ref to match the remote. Without this, the cache volume would hold a stale
  branch tip from a previous session — `git fetch` updates `origin/${BRANCH}`
  but doesn't move the local ref. Any prior-session commits were already
  exported to the host by `_dev-sync`, so the reset is safe.
- **Seed validation**: Before the `rsync --delete` overlay in the container,
  `devInitScript` checks that `/tmp/src-seed/go.mod` exists. This prevents
  wiping `/src` when `m.Source` is empty or corrupt.
- **Base branch error**: When creating a new branch fails because
  `origin/${BASE}` doesn't exist, the init script prints an actionable error
  message naming the missing ref instead of a raw git error.
- **Source overlay**: Local source files (via `m.Source`, which excludes `.git`)
  are synced on top of the checked-out branch via `rsync --delete`, bringing
  uncommitted changes (including file deletions) into the container.
- **Non-fatal go mod download**: `go mod download` runs after source overlay but
  failures are non-fatal (warning printed). This prevents malformed `go.mod` in
  local changes from blocking container startup.
- **`_dev-session` shared logic**: The `dev` and `claude` Taskfile tasks delegate
  to the internal `_dev-session` task, which handles lockfile management, cleanup
  defers, the `dagger call dev` invocation, atomic export staging, and `_dev-sync`.
  The `dev` task passes no extra args; `claude` passes `DEV_EXTRA_ARGS` with
  `--claude-config`, `--claude-json`, `--ccstatusline-config`, and `--cmd`.
- **Atomic export staging**: The Taskfile `dev` and `claude` tasks export to a
  `.partial` staging directory, then atomically `mv` it to the final path. A
  `defer` cleans up the staging directory on any exit (including ctrl+c). This
  prevents partial exports from corrupting worktrees on interruption.
- **Translation layer**: The Taskfile `_dev-sync` internal task (called by both
  `dev` and `claude`) handles converting between the container's standalone `.git`
  directory and the host's worktree format. When the export contains a `.git`
  directory, commits are imported via `git fetch` to FETCH_HEAD, then
  `git update-ref` updates the branch ref (avoiding the "refusing to fetch into
  checked-out branch" error). If the export lacks a `.git` directory, a warning
  is printed and commit import is skipped. On fetch failure, the export is
  preserved at `/tmp/dagger-dev-<dir>` for manual recovery. The worktree is then
  reset to the branch tip and `rsync --delete` overlays the container's
  uncommitted changes (including file deletions).
- **Optional BRANCH**: The `dev` and `claude` Taskfile tasks default `BRANCH` to
  the current git branch (`git branch --show-current`). If the host is in
  detached HEAD state, a precondition fails with a message asking the user to set
  `BRANCH` explicitly.
- **Branch name mapping**: Branch names are sanitized by replacing `/` with `-`
  for cache volume names and worktree directories. Branches differing only by
  `/` vs `-` (e.g., `feat/example` and `feat-example`) would collide. Avoid
  using such conflicting names simultaneously.
- **Single session per branch**: Concurrent `dev`/`claude` sessions for the same
  branch on the same host are not supported. The shared cache volume and temp
  export path (`/tmp/dagger-dev-<dir>`) assume a single active session per branch.
- **Export validation**: Before `rsync --delete` overwrites the worktree,
  `_dev-sync` checks that the export contains `go.mod`. If the export is empty
  or corrupt, the sync aborts with an error and preserves the export at
  `/tmp/dagger-dev-<dir>` for manual inspection. This prevents accidental data
  loss from a broken container export.
- **FETCH_HEAD validation**: After `git fetch`, the sync verifies that
  `FETCH_HEAD` is a valid ref before calling `git update-ref`. This prevents
  pointing a branch at a stale or corrupt ref.
- **Concurrent session lockfile**: The `dev` and `claude` tasks write a PID
  lockfile at `/tmp/dagger-dev-<dir>.lock` before starting, using `$PPID` (the
  Task process PID) rather than `$$` (which would be the transient `sh -c`
  shell). This ensures the lockfile PID stays alive for the entire task duration,
  providing correct concurrent session detection. If a lockfile exists and its
  PID is still alive, the task refuses to start. Stale lockfiles (dead PIDs) are
  automatically cleaned up with a warning. The lockfile is removed on exit via
  `defer`.
- **Branch tracking after sync**: After importing commits, `_dev-sync` sets
  `branch.<name>.remote` and `branch.<name>.merge` so that `git push` works
  without arguments in the worktree.
- **Cache volume collision**: Branch names are sanitized by replacing `/` with
  `-` for cache volume names and worktree directories. Branches differing only
  by `/` vs `-` (e.g., `feat/example` and `feat-example`) share the same cache
  volume and temp paths, causing collisions. Avoid using such conflicting names.
- **Worktree path resolution**: `_dev-sync` resolves the target worktree path
  via `git worktree list --porcelain`, matching on
  `branch refs/heads/${BRANCH}`. This produces an absolute path that works
  regardless of the caller's working directory — running `task dev` from inside
  an existing worktree works correctly because the path is resolved from git's
  tracking rather than relative to `$PWD`. When no worktree exists yet, the
  path falls back to `<repo-root>/.worktrees/<dir>` derived from
  `git rev-parse --git-common-dir`.
- **`_DEV_TS` cache-busting**: `DevEnv()` sets a `_DEV_TS` environment variable
  to `time.Now().String()`, busting the Dagger function cache on every call.
  Without it, if `m.Source` hasn't changed, Dagger would return a cached result
  and skip `git fetch origin`, so remote branch updates wouldn't be picked up.
- **Hardcoded upstream URL**: The `devInitScript` in `ci/main.go` clones from
  `github.com/macropower/kclipper.git`. Forks that change the remote URL must
  update this constant.
- **Must use Taskfile tasks**: Running raw `dagger call dev --output=.`
  would overwrite the host's `.git` worktree file. Always use `task dev` or
  `task claude` for proper worktree handling.

## Git Initialization Helpers

Containers need a `.git` directory when the host uses a worktree (where `.git`
is a file pointing to a host path that doesn't exist in the container). There
are three approaches, ordered from fastest to slowest:

- **Static `.git/HEAD` injection** — uses `Directory.WithNewFile(".git/HEAD",
...)` to create a minimal `.git` directory. This is a content-addressed
  operation with zero container exec overhead. Sufficient when the tool just
  needs to locate the repo root (e.g. `FindRepoRoot` checks for `.git/HEAD`).
  Used by `goBase()`.

- **`ensureGitInit`** — runs `git init` via a container exec. Slightly slower
  than static injection but still fast. Used by `LintCommitMsg`.

- **`ensureGitRepo`** — runs `git init`, `git add -A`, and `git commit`.
  Required when the tool inspects committed files, dirty-tree state, or version
  history. Used by `releaserBase()` (GoReleaser needs committed files for
  dirty-tree detection, version derivation, and source archives).

**Convention:** prefer static `.git/HEAD` injection for new pipelines. Use
`ensureGitInit` only when a real git repository is needed but committed files
are not. Use `ensureGitRepo` only when the tool requires committed files.

## Test Functions

- **`Test()`** — fast pre-commit check. Omits `-race` and `-vet=all` for
  speed. Race detection is redundant here because CI runs `TestCoverage()`
  (which includes `-race`), and `-vet=all` is redundant with `Lint()` (which
  runs govet via golangci-lint).

- **`TestCoverage()`** — full CI-grade test run with `-race -vet=all` and
  coverage profiling. Used in `validate.yaml` via `TestCoverageProfile` in
  the test module.

## Caching Strategy

The CI module uses a three-tier caching approach to minimize redundant work:

1. **Dagger function caching** — Dagger caches function results by default
   (7-day TTL). Functions with external side effects use `// +cache="never"`.

2. **Module pre-download layer** (`goModBase`) — The `New` constructor accepts
   a separate `goMod` directory parameter synced with
   `+ignore=["*", "!go.mod", "!go.sum"]`. This gives it a content hash
   independent of `source`, so its cache key changes only when dependency
   files change. `goModBase` copies this directory into the container and runs
   `go mod download` before mounting the full source and cache volumes. Both
   `lintBase()` and `goBase()` delegate to `goModBase`.

3. **Cache volumes** — Named Dagger cache volumes persist across runs:
   - `go-mod` — Go module cache (`GOMODCACHE`)
   - `go-build` — Go build cache (`GOCACHE`)
   - `golangci-lint` — golangci-lint analysis cache
   - `npm-cache` — npm download cache for prettier

## Taskfile Boundary

The Taskfile and Dagger module have a clear division of responsibilities:

- **Thin wrappers** — most tasks (`lint`, `test`, `build`, `format`, `check`,
  etc.) are one-line delegations to `dagger check`/`dagger call`/`dagger generate`
  for developer convenience.
- **Host orchestration** — `dev`, `claude`, `_dev-session`, `_dev-sync`, and
  worktree tasks handle operations that cannot run inside Dagger containers:
  host git state, PID lockfiles, atomic export staging, rsync to worktrees.

Rule of thumb: if it touches the host filesystem, host git, or host processes,
it belongs in the Taskfile. Everything else belongs in Dagger.

## Version Management

Tool versions are declared as constants at the top of `main.go` with Renovate
annotations for automated updates (e.g. `// renovate: datasource=... depName=...`).
Change the constant value to update a version. The Dagger engine version is
pinned in both `dagger.json` (`engineVersion`) and the `daggerVersion` constant.
