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

**Always use `task dev` or `task claude`** — raw `dagger call dev --output=.`
would overwrite the host's `.git` worktree file.

Container behaviors (see `devInitScript` for details):

- Blobless clone (`--filter=blob:none`) for full history with on-demand blobs.
- Per-branch cache volumes (`dev-src-<branch>`) for branch isolation.
- `_DEV_TS` env var busts Dagger's function cache so `git fetch` always runs.
- Non-fatal `git fetch` allows offline use when a local cache exists.
- Force checkout (`-f`) avoids conflicts from stale cache volume files; safe
  because `rsync --delete` overlays host source immediately after.
- Source overlay via `rsync --delete` brings uncommitted changes (including
  deletions) into the container.
- Non-fatal `go mod download` prevents malformed `go.mod` from blocking startup.
- Claude config: host `~/.claude` seeded into a global cache volume via
  `rsync -a` (without `--delete`), so auth tokens persist across sessions.

Taskfile host orchestration (`_dev-session`, `_dev-sync`):

- Atomic export staging (`.partial` dir, then `mv`) prevents partial exports.
- `_dev-sync` translates container `.git` → host worktree format via
  `git fetch` + `git update-ref` (avoids "refusing to fetch into checked-out
  branch"). Failed exports preserved at `/tmp/dagger-dev-<dir>`.
- PID lockfile (`$PPID`, not `$$`) prevents concurrent sessions per branch.
- Branch names sanitized (`/` → `-`) for volume/path names. Branches differing
  only by `/` vs `-` collide — avoid such conflicting names.
- Single session per branch: concurrent `dev`/`claude` for the same branch is
  not supported.

## Git Initialization Helpers

Containers need a `.git` directory when the host uses a worktree. Three
approaches, fastest to slowest:

- **Static `.git/HEAD` injection** — `Directory.WithNewFile(".git/HEAD", ...)`.
  Zero exec overhead. Use when the tool just needs to locate the repo root.
- **`ensureGitInit`** — runs `git init`. Use when a real git repo is needed
  but committed files are not.
- **`ensureGitRepo`** — runs `git init`, `git add -A`, `git commit`. Use when
  the tool requires committed files (e.g. GoReleaser).

**Convention:** prefer static injection for new pipelines.

## Test Functions

- **`Test()`** — runs the Go test suite without `-coverprofile`, using only
  cacheable flags so that Go's internal test result cache (`GOCACHE`) can
  skip unchanged packages across runs via the persistent `go-build` cache
  volume. On iterative local development, only affected packages re-run.

- **`TestCoverage()`** — runs Go tests with `-coverprofile` and returns the
  coverage profile file. Runs independently of `Test()` because
  `-coverprofile` disables Go's test result caching. Dagger's layer caching
  still shares the base container layers (image, module download) with
  `Test()`.

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

3. **Cache volumes** — Named Dagger cache volumes persist across runs.
   Volume names include tool versions so that version bumps start fresh:
   - `go-mod-<goVersion>` — Go module cache (`GOMODCACHE`)
   - `go-build-<goVersion>` — Go build cache (`GOCACHE`)
   - `golangci-lint-<lintVersion>` — golangci-lint analysis cache
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
