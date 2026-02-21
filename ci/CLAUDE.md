# CI Module

Dagger-based CI/CD module for kclipper. All CI tasks (testing, linting,
formatting, building, releasing) run in containers orchestrated by Dagger.

## Quick Reference

```bash
dagger check                  # Run all checks (+check functions)
dagger check lint             # Run specific check(s)
dagger generate --auto-apply  # Run generators and apply changes
dagger call build export --path=./dist  # Build binaries
dagger call dev terminal      # Interactive dev container
```

Or via the Taskfile:

```bash
task test      # dagger check test
task lint      # dagger check lint lint-prettier lint-actions lint-releaser
task format    # dagger generate --auto-apply
task build     # dagger call build export --path=./dist
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
```

### Function Categories

| Category    | Annotation   | CLI                    | Purpose                                    |
| ----------- | ------------ | ---------------------- | ------------------------------------------ |
| Checks      | `// +check`  | `dagger check <name>`  | Validation (tests, lints). Return `error`. |
| Generators  | `// +generate` | `dagger generate`    | Code formatting. Return `*dagger.Changeset`. |
| Build       | (none)       | `dagger call <name>`   | Artifact production.                       |
| Development | (none)       | `dagger call dev terminal` | Interactive containers.                |

### Source Directory Filtering

The `New` constructor uses `+ignore` to exclude directories that are never
needed inside CI containers (`dist`, `.worktrees`, `.tmp`, `.devcontainer`).
This reduces the context transfer size when invoking Dagger functions.

### Base Container Pattern

Private helper methods build reusable container layers:

- `prettierBase()` -- Node + prettier (standalone; callers mount source).
- `goBase()` -- Go toolchain + source + module/build caches.
- `lintBase()` -- golangci-lint Alpine image + linux-headers + caches.
- `releaserBase()` -- Go + GoReleaser + Zig (CGO cross-compilation) + cosign + syft + macOS SDK.

All Go-based base containers share the same cache volumes (`go-mod`,
`go-build`) and set `GOMODCACHE`/`GOCACHE` environment variables to match.

### Git Worktree Handling

`ensureGitRepo()` detects when the source comes from a git worktree (where
`.git` is a file referencing a host path that doesn't exist in the container)
and initializes a minimal git repo so GoReleaser and tests work correctly.

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

2. Add to the lint task in `Taskfile.yaml`:

```yaml
lint:
  cmds:
    - dagger check lint lint-prettier lint-actions lint-releaser lint-foo
```

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
- Cross-compilation macOS SDK flags are defined in the `macosSDKFlags`
  constant to avoid repetition across `CC_*`/`CXX_*` env vars.
- The `publishImages` helper returns `[]string` digests directly rather than
  formatted strings, keeping callers clean.

## Version Management

Tool versions are declared as constants at the top of `main.go` with Renovate
annotations for automated updates:

```go
const (
    goVersion = "1.25" // renovate: datasource=golang-version depName=go
)
```

When updating a version:
- Change the constant value.
- The Renovate comment tells the bot which datasource and package to track.
- The Dagger engine version is pinned in both `dagger.json` (`engineVersion`)
  and the `daggerVersion` constant (for the dev container).

## Caching Strategy

All Go-based containers use shared Dagger cache volumes:

| Volume Key        | Mount Path          | Env Variable   |
| ----------------- | ------------------- | -------------- |
| `go-mod`          | `/go/pkg/mod`       | `GOMODCACHE`   |
| `go-build`        | `/go/build-cache`   | `GOCACHE`      |
| `golangci-lint`   | `/root/.cache/golangci-lint` | (implicit) |

Always set the corresponding env var when mounting a cache so the tool
actually uses it.

## Cross-Compilation

Builds target four platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`,
`darwin/arm64`. CGO cross-compilation uses Zig as the C/C++ compiler via
wrapper scripts in `hack/`:

- `hack/zig-gold-wrapper.sh` -- Linux targets
- `hack/zig-macos-wrapper.sh` -- macOS targets (uses vendored SDK headers from `.nixpkgs/vendor/`)

The `CC_*`/`CXX_*` env vars in `releaserBase()` are consumed by GoReleaser's
`.goreleaser.yaml` per-target `env` blocks.

## Container Images

Runtime images are built natively via Dagger (not Docker-in-Docker):

- Base: `debian:13-slim`
- Multi-arch: `linux/amd64` + `linux/arm64`
- Published to `ghcr.io/macropower/kclipper`
- Optionally signed with cosign

GoReleaser's Docker support is intentionally skipped (`--skip=docker`) in
favor of Dagger's `Container.Publish()` for proper multi-arch manifests.

## GitHub Workflows

| Workflow         | Trigger               | Dagger Usage                          |
| ---------------- | --------------------- | ------------------------------------- |
| `validate.yaml`  | Push/PR               | `dagger check` (lint) + `dagger call test-coverage` |
| `build.yaml`     | Push to main, PR      | `dagger call build`                   |
| `release.yaml`   | Tag push `v*`         | `dagger call release`                 |

All workflows use `dagger/dagger-for-github@v8` with version `"0.19"`.
