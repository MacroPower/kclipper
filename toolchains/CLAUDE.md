# CI Module

Dagger-based CI/CD for kclipper. All CI tasks (testing, linting, formatting,
building, releasing) run in containers orchestrated by Dagger.

## Quick Reference

```bash
dagger check                          # Run all checks (+check functions)
dagger check go:lint                  # Run specific check(s)
dagger call go lint-deadcode          # Run deadcode analysis (opt-in, advisory)
dagger generate --auto-apply          # Run generators (Format + Generate) and apply changes
dagger call kclipper build --output=./dist       # Build binaries
task dev                              # Dev container (branch defaults to current)
task dev BRANCH=feat/foo              # Dev container with explicit branch
task claude                           # Claude Code container (branch defaults to current)
task claude BRANCH=feat/foo           # Claude Code container with explicit branch
dagger call go lint-commit-msg --msg-file=.git/COMMIT_EDITMSG  # Validate commit message
```

## Architecture

Three Dagger toolchain modules work together:

- **`go`** (toolchain) — Reusable Go CI module with generic functions (test,
  lint, format, publish). Lives at `toolchains/go/` with its own `dagger.json`.
  Can be consumed by any Go project. Carries `+check` and `+generate`
  annotations directly.
- **`dev`** (toolchain) — Reusable development container module (DevBase,
  DevEnv, Dev). Lives at `toolchains/dev/` with its own `dagger.json`. Fully
  independent from the `go` module.
- **`kclipper`** (toolchain) — Kclipper-specific CI layer that depends on `go`
  and `dev` and adds project-specific functions (build, release, runtime
  images, dev container wrappers). Lives at `toolchains/kclipper/`.

```
dagger.json          # Root module config (toolchains + customizations)
toolchains/
  go/
    dagger.json      # Reusable module config (name=go)
    main.go          # Struct, constructor, constants, base containers, helpers
    check.go         # Test, TestCoverage, Lint, LintPrettier, LintActions, LintReleaser, LintDeadcode, LintCommitMsg
    generate.go      # Format, Generate
    publish.go       # PublishImages, VersionTags, FormatDigestChecksums, DeduplicateDigests, RegistryHost
    bench.go         # BenchmarkResult, Benchmark, BenchmarkSummary, CacheBust, benchmarkStages, runBenchmarks
    tests/           # Generic function tests
  dev/
    dagger.json      # Reusable module config (name=dev)
    main.go          # Dev container functions
    tests/           # Dev container tests
  kclipper/
    dagger.json      # Kclipper-specific (depends on go + dev)
    main.go          # Struct, constructor, constants, goToolchain, devToolchain, LintReleaser
    build.go         # Build, BuildImages, runtimeImages, runtimeBase, releaserBase
    publish.go       # PublishImages, Release
    bench.go         # BenchmarkResult, Benchmark, BenchmarkSummary, benchmarkStages, runBenchmarks
    dev.go           # DevEnv, Dev
    tests/           # Kclipper-specific integration tests
```

The root `dagger.json` registers `go` and `kclipper` as toolchains with
`customizations` that declare source ignore patterns (e.g. `dist`,
`.worktrees`, `.tmp`, `.git`) at the project level. The `kclipper` module
depends on both `go` and `dev` via local path dependencies and delegates
generic CI work to those modules.

### What lives where

| `go` toolchain (CI)                                                  | `dev` toolchain (dev containers)  | `kclipper` toolchain (kclipper-specific)                            |
| -------------------------------------------------------------------- | --------------------------------- | ------------------------------------------------------------------- |
| Test (+check), TestCoverage                                          | DevBase, DevEnv, Dev              | Build, Release                                                      |
| Lint (+check), LintPrettier (+check), LintActions (+check)           | applyDevConfig, devToolBins       | BuildImages, PublishImages (builds images, delegates publish to go) |
| LintReleaser, LintDeadcode, LintCommitMsg                            | claudeCodeFiles, sanitizeCacheKey | LintReleaser (+check, uses kclipper clone URL)                      |
| Format (+generate), Generate (+generate)                             | Shell/tool version constants      | releaserBase (extends ReleaserBase with Zig, macOS SDK, KCL LSP)    |
| GoBase, LintBase, PrettierBase, GoModBase, ReleaserBase              | starshipConfig, zshConfig         | runtimeImages, runtimeBase (KCL env, OCI labels)                    |
| EnsureGitInit, EnsureGitRepo                                         | devInitScript                     | DevEnv/Dev wrappers (pass clone URL)                                |
| PublishImages (publish+sign workflow)                                |                                   | Benchmark/BenchmarkSummary (add build stage)                        |
| VersionTags, FormatDigestChecksums, DeduplicateDigests, RegistryHost |                                   |                                                                     |
| Benchmark (generic stages), BenchmarkSummary                         |                                   |                                                                     |
| CI version constants (Go, golangci-lint, prettier, cosign, syft)     |                                   | Project-specific versions (Zig, KCL LSP)                            |

### Function Categories

| Category    | Annotation     | CLI                                   | Purpose                                        |
| ----------- | -------------- | ------------------------------------- | ---------------------------------------------- |
| Checks      | `// +check`    | `dagger check <toolchain>:<name>`     | Validation (tests, lints). Return `error`.     |
| Generators  | `// +generate` | `dagger generate`                     | Code formatting. Return `*dagger.Changeset`.   |
| Build       | (none)         | `dagger call kclipper <name>`         | Artifact production.                           |
| Callable    | (none)         | `dagger call <toolchain> <name>`      | Requires arguments; invoked via `dagger call`. |
| Development | (none)         | `dagger call kclipper dev --output=.` | Interactive container with persistent export.  |
| Testable    | (none)         | `dagger call <toolchain> <name>`      | Non-interactive building blocks for tests.     |

`LintCommitMsg` is in the Callable category because it requires a mandatory
`msgFile` argument and cannot be a `+check` function.

`LintDeadcode` is in the Callable category as an opt-in advisory lint. It is
not a `+check` function so it does not run during `dagger check`.

`DevEnv` and `DevBase` are in the Testable category: they expose intermediate
pipeline stages that the test module exercises without needing `Terminal()`.

## Adding a New Check

For **generic CI** checks (useful to any Go project), add to `toolchains/go/check.go`.
For **dev container** functions, add to `toolchains/dev/main.go`.
For **kclipper-specific** checks, add to `toolchains/kclipper/main.go` (delegating
to the appropriate toolchain where possible).

1. Add a public method with `// +check` annotation:

```go
// LintFoo checks foo.
//
// +check
func (m *Kclipper) LintFoo(ctx context.Context) error {
    return m.goToolchain().LintFoo(ctx)  // delegate to go toolchain
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
func (m *Go) GenerateFoo() *dagger.Changeset {
    // ...
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
- Functions with external side effects (publishing, releasing) use
  `// +cache="never"`. Functions that should re-run each session (Test,
  Benchmark) use `// +cache="session"`. Deterministic functions use
  Dagger's default function caching.
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
history. The kclipper module's `Dev()` wraps `dev.Dev()` with the kclipper
clone URL. The dev toolchain module handles the generic dev container setup:
tool installation (`DevBase`), git clone + source overlay (`DevEnv`), and
interactive terminal + export (`Dev`).

**Always use `task dev` or `task claude`** — raw `dagger call kclipper dev --output=.`
would overwrite the host's `.git` worktree file.

Container behaviors (see `devInitScript` in `toolchains/dev/main.go` for details):

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
- **`EnsureGitInit`** — runs `git init`. Use when a real git repo is needed
  but committed files are not.
- **`EnsureGitRepo`** — runs `git init`, `git add -A`, `git commit`. Use when
  the tool requires committed files (e.g. GoReleaser).

These helpers live in the `go` toolchain and are called via
`m.goToolchain().EnsureGitInit(...)`.

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
   (7-day TTL). Three cache tiers are used:

   - **Default** — deterministic functions (lint, format) use the 7-day TTL.
   - **`+cache="session"`** — functions that should re-run each session but
     not be cached across sessions (Test, Benchmark, BenchmarkSummary).
   - **`+cache="never"`** — functions with external side effects (PublishImages,
     Release, Dev) that must never return stale results.

2. **Module pre-download layer** (`GoModBase`) — The constructor accepts a
   separate `goMod` directory parameter synced with
   `+ignore=["*", "!go.mod", "!go.sum"]`. This gives it a content hash
   independent of `source`, so its cache key changes only when dependency
   files change. `GoModBase` copies this directory into the container and runs
   `go mod download` before mounting the full source and cache volumes. Both
   `LintBase()` and `GoBase()` delegate to `GoModBase`.

3. **Cache volumes** — Named Dagger cache volumes persist across runs.
   Volume names include tool versions so that version bumps start fresh.
   The Go toolchain constructor accepts optional `moduleCache` and
   `buildCache` parameters; when omitted, versioned defaults are used:
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

Tool versions are declared as constants at the top of each module's `main.go`
with Renovate annotations for automated updates (e.g.
`// renovate: datasource=... depName=...`).

- **CI tool versions** (Go, golangci-lint, prettier, cosign, syft, etc.) are
  in `toolchains/go/main.go`. The Go version is also configurable via the
  constructor's optional `goVersion` parameter (defaults to `defaultGoVersion`).
- **Dev tool versions** (task, lefthook, dagger, starship, yq, uv, gh,
  claude-code) are in `toolchains/dev/main.go`.
- **Project-specific versions** (Zig, KCL LSP) are in
  `toolchains/kclipper/main.go`.

The Dagger engine version is pinned in `dagger.json` (`engineVersion`) and
the `daggerVersion` constant in `toolchains/dev/main.go`.
