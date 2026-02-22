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
task test:integration  # dagger check -m ci/tests
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

### Build & Image Functions

- `Build()` -- GoReleaser snapshot build, returns `dist/` directory.
- `BuildImages(version, dist)` -- Multi-arch container images from GoReleaser dist.
- `VersionTags(tag)` -- Derives image tags from a version string.
- `FormatDigestChecksums(refs)` -- Converts publish output to checksums format.
- `DeduplicateDigests(refs)` -- Unique image references by sha256 digest.
- `RegistryHost(registry)` -- Extracts host from a registry address.
- `PublishImages(tags, ..., cosignPassword, dist)` -- Build, publish, and optionally sign images.
- `Release(tag, ..., cosignPassword, ...)` -- Full release pipeline (GoReleaser + image publish).

The `New` constructor accepts an optional `registry` parameter (defaults to
`ghcr.io/macropower/kclipper`). Override for testing:
`dag.Ci(dagger.CiOpts{Registry: "ttl.sh/test"})`.

### Source Directory Filtering

The `New` constructor uses `+ignore` to exclude directories that are never
needed inside CI containers (`dist`, `.worktrees`, `.tmp`).
This reduces the context transfer size when invoking Dagger functions.

### Base Container Pattern

Private helper methods build reusable container layers:

- `prettierBase()` -- Node + prettier (standalone; callers mount source).
- `goToolchain()` -- Configures `dag.Go()` with project source, Go version,
  caches, and CGO. Returns `*dagger.Go` used by `goBase()`.
- `goBase()` -- Wolfi-based container via `goToolchain().Env()` + `ensureGitRepo()`.
- `lintBase()` -- golangci-lint Alpine image + linux-headers + caches.
- `releaserBase()` -- Go + GoReleaser + Zig + cosign + syft + KCL LSP binaries
  - macOS SDK. Tool binaries extracted from OCI images via `Container.File()`.

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
dagger check -m ci/tests              # Run all integration tests via +check
dagger call -m ci/tests all           # Run all tests via the All runner
dagger call -m ci/tests test-source-filtering   # Run a specific test
```

Tests are Dagger Functions annotated with `+check` that accept
`context.Context` and return `error` for pass/fail. Using `dagger check`
runs them concurrently via Dagger's built-in check runner. The `All`
function provides an alternative that runs tests in parallel using
`errgroup`. To add a new test, add a `+check`-annotated method on
`Tests` and register it in `All`.

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
- Pure-logic utilities (`FormatDigestChecksums`, `DeduplicateDigests`,
  `VersionTags`, `RegistryHost`) are public methods directly, with no
  private helper indirection. Internal callers use `m.MethodName(...)`.
  This keeps the logic in one place while remaining testable from the test
  module (since `go test` cannot run on Dagger modules).
- Registry auth (`WithRegistryAuth`) is conditional: only applied when
  `registryPassword` is non-nil. This allows `PublishImages` to work with
  anonymous registries (e.g. ttl.sh) for integration testing.
- Use `env://VAR_NAME` syntax for Dagger secret providers on the CLI
  (e.g. `--token=env://GITHUB_TOKEN`).
- Use `errgroup` for concurrent execution of independent operations
  (e.g. signing, test execution).
- Tool binaries are extracted from OCI images via `Container.File()`
  (not `go install`) for faster builds and automatic platform matching.
- Published container images include OCI labels and annotations for
  metadata discoverability.
- Cosign key-based signing (`--key env://COSIGN_KEY`) is used for both
  binary artifacts and container images. Keyless OIDC is not available
  inside Dagger containers.

## Version Management

Tool versions are declared as constants at the top of `main.go` with Renovate
annotations for automated updates:

```go
const (
    goVersion       = "1.25"    // renovate: datasource=golang-version depName=go
    starshipVersion = "v1.24.2" // renovate: datasource=github-releases depName=starship/starship
    yqVersion       = "v4.52.4" // renovate: datasource=github-releases depName=mikefarah/yq
    uvVersion       = "0.10.4"  // renovate: datasource=github-releases depName=astral-sh/uv
    ghVersion       = "v2.87.2" // renovate: datasource=github-releases depName=cli/cli
)
```

When updating a version:

- Change the constant value.
- The Renovate comment tells the bot which datasource and package to track.
- The Dagger engine version is pinned in both `dagger.json` (`engineVersion`)
  and the `daggerVersion` constant (for the dev container).

## Caching Strategy

All Go-based containers share Dagger cache volumes. The Go toolchain
(`goBase`) manages mount paths automatically; other containers mount explicitly:

| Volume Key      | Mount Path (toolchain) | Mount Path (manual)          | Env Variable |
| --------------- | ---------------------- | ---------------------------- | ------------ |
| `go-mod`        | (managed by toolchain) | `/go/pkg/mod`                | `GOMODCACHE` |
| `go-build`      | (managed by toolchain) | `/go/build-cache`            | `GOCACHE`    |
| `golangci-lint` | —                      | `/root/.cache/golangci-lint` | (implicit)   |
| `shell-history` | —                      | `/commandhistory`            | `HISTFILE`   |

Different mount paths are safe because Go caches are content-addressed.
When mounting manually, always set the corresponding env var so the tool
actually uses it.

## Cross-Compilation

Builds target `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.
CGO cross-compilation uses Zig via wrapper scripts (`hack/zig-gold-wrapper.sh`
for Linux, `hack/zig-macos-wrapper.sh` for macOS). The `CC_*`/`CXX_*` env vars
in `releaserBase()` are consumed by GoReleaser's `.goreleaser.yaml` per-target
`env` blocks. macOS SDK flags are defined in the `macosSDKFlags` constant.

## Development Containers

The `Dev()` function creates an interactive development container with zsh,
starship prompt, and a curated set of tools pre-installed:

- **Core**: Go, Task, conform, lefthook, Dagger CLI, Claude Code, gh
- **Shell**: zsh with zsh-autosuggestions, zsh-syntax-highlighting, starship
  prompt, and direnv
- **CLI utilities**: ripgrep (`rg`), fd-find (`fd`), bat, fzf, tree, htop,
  yq, jq, uv, dnsutils

Optional config directories can be bind-mounted for Claude Code, git, and
ccstatusline. Shell history is persisted via a `shell-history` cache volume.
`ExperimentalPrivilegedNesting` is enabled on the terminal command so nested
`dagger call`/`dagger check` invocations work inside the container without
Docker socket mounting.

### Dev Container Safety

AI agents run inside the Dev container with `ExperimentalPrivilegedNesting`
enabled and can safely use `dagger` commands. The container is isolated from
the host machine:

| Property                   | Detail                                                                  |
| -------------------------- | ----------------------------------------------------------------------- |
| `host { directory }` scope | Container filesystem only (not host machine)                            |
| Docker socket              | Not mounted                                                             |
| Root capabilities          | Standard runc sandboxing (no `InsecureRootCapabilities`)                |
| Mounted directories        | Dagger snapshots (not live bind mounts; writes don't propagate to host) |
| Network                    | Outbound only                                                           |
| Cache volumes              | Content-addressed, shared safely across containers                      |

Note: The Dagger SDK docs still warn that `ExperimentalPrivilegedNesting`
grants "FULL ACCESS TO YOUR HOST FILESYSTEM." This warning predates
sandboxing improvements (branching from dagger/dagger#6916). This has been
verified empirically.

## GitHub Workflows

| Workflow        | Trigger          | Dagger Usage                                                      |
| --------------- | ---------------- | ----------------------------------------------------------------- |
| `validate.yaml` | Push/PR          | `dagger check` (lint) + `dagger call test-coverage` + test module |
| `build.yaml`    | Push to main, PR | `dagger call build`                                               |
| `release.yaml`  | Tag push `v*`    | `dagger call release`                                             |

Secrets are passed via the `env://VAR_NAME` provider syntax.
