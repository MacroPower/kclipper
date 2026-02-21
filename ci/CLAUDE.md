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
task test              # dagger check test
task test:integration  # dagger call -m ci/tests all
task lint              # dagger check lint lint-prettier lint-actions lint-releaser
task format            # dagger generate --auto-apply
task build             # dagger call build export --path=./dist
task dev               # Interactive dev shell via Dagger
task claude            # Claude Code inside Dagger dev container
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

### Dependencies

The module depends on the [Dagger Go toolchain](https://github.com/dagger/dagger/tree/main/toolchains/go)
(`github.com/dagger/dagger/toolchains/go`), installed as a module dependency
(not a Dagger toolchain). This provides `dag.Go()` for constructing Go build
environments with Wolfi-based containers, automatic cache management, and
CGO support. It is used by `goToolchain()` / `goBase()` only; `lintBase()`,
`releaserBase()`, and `Dev()` remain manually configured due to specialized
requirements. To update the dependency pin: `dagger update go`.

### Function Categories

| Category    | Annotation     | CLI                        | Purpose                                      |
| ----------- | -------------- | -------------------------- | -------------------------------------------- |
| Checks      | `// +check`    | `dagger check <name>`      | Validation (tests, lints). Return `error`.   |
| Generators  | `// +generate` | `dagger generate`          | Code formatting. Return `*dagger.Changeset`. |
| Build       | (none)         | `dagger call <name>`       | Artifact production.                         |
| Development | (none)         | `dagger call dev terminal` | Interactive containers.                      |

### Source Directory Filtering

The `New` constructor uses `+ignore` to exclude directories that are never
needed inside CI containers (`dist`, `.worktrees`, `.tmp`, `.devcontainer`).
This reduces the context transfer size when invoking Dagger functions.

### Base Container Pattern

Private helper methods build reusable container layers:

- `prettierBase()` -- Node + prettier (standalone; callers mount source).
- `goToolchain()` -- Configures the Dagger Go toolchain dependency (`dag.Go()`)
  with the project source, Go version, cache volumes, and CGO enabled. Returns
  a `*dagger.Go` instance used by `goBase()`.
- `goBase()` -- Calls `goToolchain().Env()` to get a Wolfi-based container with
  Go, source, and caches pre-configured, then wraps it with `ensureGitRepo()`.
- `lintBase()` -- golangci-lint Alpine image + linux-headers + caches. Independent
  of the Go toolchain because we pin a specific golangci-lint version via Renovate.
- `releaserBase()` -- Go + GoReleaser + Zig (CGO cross-compilation) + cosign + syft + macOS SDK.

All Go-based base containers share the same cache volumes (`go-mod`,
`go-build`). The toolchain-based containers (`goBase`) mount them at
toolchain-default paths, while `lintBase`, `releaserBase`, and `Dev` mount
them explicitly. This is safe because Go caches are content-addressed.

### Git Worktree Handling

`ensureGitRepo()` detects when the source comes from a git worktree (where
`.git` is a file referencing a host path that doesn't exist in the container)
and initializes a minimal git repo so GoReleaser and tests work correctly.

### Test Module

`ci/tests/` is a separate Dagger module that imports the CI module as a
dependency and tests its public API. This is the Dagger-recommended pattern
since `go test` cannot run directly on Dagger modules (the generated SDK's
`init()` requires a running Dagger session).

```bash
dagger call -m ci/tests all            # Run all integration tests
dagger call -m ci/tests test-source-filtering   # Run a specific test
```

Tests are Dagger Functions that accept `context.Context` and return `error`
for pass/fail. The `All` function runs all tests in parallel using
`errgroup`. To add a new test, add a method on `Tests` and register it
in `All`.

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
- The package-level doc comment in `main.go` provides the module description
  shown in `dagger functions` and `dagger call --help`.
- Functions with external side effects (publishing, releasing) must use
  `// +cache="never"` to prevent stale cached results.
- Cross-compilation macOS SDK flags are defined in the `macosSDKFlags`
  constant to avoid repetition across `CC_*`/`CXX_*` env vars.
- The `publishImages` helper returns `[]string` digests directly rather than
  formatted strings, keeping callers clean.
- Published container images include standard OCI labels
  (`org.opencontainers.image.*`) for metadata discoverability.
- Use `env://VAR_NAME` syntax for Dagger secret providers on the CLI
  (e.g. `--token=env://GITHUB_TOKEN`). This is the canonical URI scheme
  documented for Dagger v0.19+.

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

### Function Caching

Dagger v0.19.4+ supports function-level caching via `+cache` annotations.
The default policy is a 7-day TTL, which works well for input-keyed
functions (checks, builds) since they cache-miss automatically when the
source changes. Functions with external side effects use `+cache="never"`:

| Function          | Cache Policy        | Reason                                     |
| ----------------- | ------------------- | ------------------------------------------ |
| Checks/Generators | (default 7-day TTL) | Input-keyed by source; auto-invalidates.   |
| `PublishImages`   | `+cache="never"`    | Publishes to registry (side effect).       |
| `Release`         | `+cache="never"`    | Creates GitHub release + publishes images. |
| `Dev`             | `+cache="never"`    | Interactive; should always be fresh.       |

### Volume Caching

All Go-based containers use shared Dagger cache volumes. The Go toolchain
(`goBase`) manages mount paths automatically; other containers mount explicitly:

| Volume Key      | Mount Path (toolchain) | Mount Path (manual)          | Env Variable |
| --------------- | ---------------------- | ---------------------------- | ------------ |
| `go-mod`        | (managed by toolchain) | `/go/pkg/mod`                | `GOMODCACHE` |
| `go-build`      | (managed by toolchain) | `/go/build-cache`            | `GOCACHE`    |
| `golangci-lint` | —                      | `/root/.cache/golangci-lint` | (implicit)   |

Different mount paths are safe because Go caches are content-addressed.
When mounting manually, always set the corresponding env var so the tool
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
- OCI labels: `org.opencontainers.image.{title,description,source,url,licenses}`
- Optionally signed with cosign

GoReleaser's Docker support is intentionally skipped (`--skip=docker`) in
favor of Dagger's `Container.Publish()` for proper multi-arch manifests.

## Development Containers

### Dagger Dev Container (`Dev`)

The `Dev()` function creates an interactive development container with Go,
Task, conform, lefthook, Dagger CLI, and Claude Code pre-installed. Optional
config directories can be bind-mounted for Claude Code and git:

```bash
task dev       # Git config only
task claude    # Git + Claude config, launches Claude Code directly
```

`ExperimentalPrivilegedNesting` is enabled so nested `dagger call` invocations
work inside the container without Docker socket mounting.

### VS Code DevContainer

`.devcontainer/` provides a VS Code Dev Container configuration using the
same Go version and tools as the Dagger dev container. It mounts Claude Code
config, git config, and uses Docker named volumes for Go caches. The
`HOST_HOME` build arg creates a symlink so host-absolute paths in mounted
configs resolve correctly inside the container.

## GitHub Workflows

| Workflow        | Trigger          | Dagger Usage                                                       |
| --------------- | ---------------- | ------------------------------------------------------------------ |
| `validate.yaml` | Push/PR          | `dagger check` (lint) + `dagger call test-coverage` + test module  |
| `build.yaml`    | Push to main, PR | `dagger call build`                                                |
| `release.yaml`  | Tag push `v*`    | `dagger call release`                                              |

All workflows use `dagger/dagger-for-github@v8` with version `"0.19"`.
Secrets are passed via the `env://VAR_NAME` provider syntax.

### Release Attestation

The release workflow creates build provenance attestations using
`actions/attest-build-provenance@v3`:

- **checksums.txt** -- GoReleaser-generated SHA256 checksums of release
  binaries and archives. Already in the standard checksums format.
- **digests.txt** -- Container image digests written by the `Release`
  function. `formatDigestChecksums()` converts Dagger's
  `registry/image:tag@sha256:hex` output to the `hex  name` format
  expected by `subject-checksums`, deduplicating by digest.
