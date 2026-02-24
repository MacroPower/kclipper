# CI Module

Dagger-based CI/CD module for kclipper. All CI tasks (testing, linting,
formatting, building, releasing) run in containers orchestrated by Dagger.

## Quick Reference

```bash
dagger check                  # Run all checks (+check functions)
dagger check lint             # Run specific check(s)
dagger generate --auto-apply  # Run generators and apply changes
dagger call build export --path=./dist  # Build binaries
task dev BRANCH=feat/foo    # Dev container (auto-creates worktree)
task claude BRANCH=feat/foo # Claude Code container (auto-creates worktree)
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

The module uses the [Dagger Go toolchain](https://github.com/dagger/dagger/tree/main/toolchains/go)
(`dag.Go()`) for Go build environments. Update pin: `dagger update go`.

`ci/tests/` is a separate Dagger module that imports CI and tests its public
API. To add a test: add a `+check`-annotated method on `Tests` and register it
in `All`.

### Function Categories

| Category    | Annotation     | CLI                               | Purpose                                        |
| ----------- | -------------- | --------------------------------- | ---------------------------------------------- |
| Checks      | `// +check`    | `dagger check <name>`             | Validation (tests, lints). Return `error`.     |
| Generators  | `// +generate` | `dagger generate`                 | Code formatting. Return `*dagger.Changeset`.   |
| Build       | (none)         | `dagger call <name>`              | Artifact production.                           |
| Callable    | (none)         | `dagger call <name>`              | Requires arguments; invoked via `dagger call`. |
| Development | (none)         | `dagger call dev export --path=.` | Interactive container with persistent export.  |

`LintCommitMsg` is in the Callable category because it requires a mandatory
`msgFile` argument and cannot be a `+check` function.

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
history. Key behaviors:

- **Blobless clone**: The upstream repo is cloned with `--filter=blob:none`,
  providing full commit history while fetching file contents on demand.
- **Branch isolation**: Each branch gets its own Dagger cache volume
  (`dev-src-<branch>`), so switching branches doesn't clobber in-progress work.
- **Source overlay**: Local source files (via `m.Source`, which excludes `.git`)
  are synced on top of the checked-out branch via `rsync --delete`, bringing
  uncommitted changes (including file deletions) into the container.
- **Translation layer**: The Taskfile `_dev-sync` internal task (called by both
  `dev` and `claude`) handles converting between the container's standalone `.git`
  directory and the host's worktree format. When the export contains a `.git`
  directory, commits are imported via `git fetch` to FETCH_HEAD, then
  `git update-ref` updates the branch ref (avoiding the "refusing to fetch into
  checked-out branch" error). The worktree is then reset to the branch tip and
  `rsync --delete` overlays the container's uncommitted changes (including file
  deletions).
- **Single session per branch**: Concurrent `dev`/`claude` sessions for the same
  branch on the same host are not supported. The shared cache volume and temp
  export path (`/tmp/dagger-dev-<dir>`) assume a single active session per branch.
- **Must use Taskfile tasks**: Running raw `dagger call dev export --path=.`
  would overwrite the host's `.git` worktree file. Always use `task dev` or
  `task claude` with a `BRANCH` argument for proper worktree handling.

## Version Management

Tool versions are declared as constants at the top of `main.go` with Renovate
annotations for automated updates (e.g. `// renovate: datasource=... depName=...`).
Change the constant value to update a version. The Dagger engine version is
pinned in both `dagger.json` (`engineVersion`) and the `daggerVersion` constant.
