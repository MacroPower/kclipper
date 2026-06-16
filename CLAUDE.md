# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`kclipper` is a superset of KCL that integrates Helm chart management. It provides KCL plugins, packages, and CLI commands to manage Helm charts declaratively and render them directly within KCL code. The binary is named `kcl` and can be used as a drop-in replacement.

## Build & Test Commands

```bash
task format    # Format and tidy code, run generators
task lint      # golangci-lint, go mod tidy check, prettier
task test      # Run tests (cgo + netgo, on the devbox PATH)
task check     # Local gate: lint + test (tools on the devbox PATH, no Dagger)
task check:all # Everything CI runs (adds security, GitHub config, releaser, via Dagger)
```

Devbox provides all required tools on PATH automatically (including gcc for the
cgo build). Go build/test/lint run locally; CI reproduces them inside the devbox
environment via the `ci` Dagger toolchain (`dagger call ci <task>`), so local and
CI execute identical commands. The `ci` module composes the shared toolchains
from github.com/MacroPower/x (devbox, goreleaser, security, zizmor), pinned in
`ci/dagger.json` and the root `dagger.json`.

kclipper-specific tasks (build, release, KCL docs, host benchmarks) live in the
`kcl:` namespace, e.g. `task kcl:build`. Run `task -l` for the full list. The
release pipeline (cgo + Zig cross-compile, KCL language-server bundling, macOS
notarization, KCL module publishing) lives in the `ci` module; see
`ci/CLAUDE.md`.

## Code Style

### Go Conventions

- Document all exported items with doc comments.
- Package documentation in `doc.go` files.
- Wrap errors with `fmt.Errorf("context: %w", err)`, or `fmt.Errorf("%w: %w", ErrSentinel, err)`.
- Avoid using "failed" or "error" in library error messages.
- Use global error variables for common errors.
- Use constructors with functional options.
- Accept interfaces, return concrete types.
- Prefer consistency over performance, avoid "fast paths" that could lead to unpredictable behavior.

### Documentation

- Use `[Name]` syntax for Go doc links. Use `[*Name]` for pointer types.
- Constructors should always begin: `// NewThing creates a new [Thing].`
- Types with constructors should always note: `// Create instances with [NewThing].`
- Interfaces should note: `// See [Thing] for an implementation.`
- Interfaces should have sensible names: `type Builder interface { Build() Thing } // Builder builds [Thing]s.`
- Functional option types should have a list linking to all functions of that type.
- Functional options should always have a link to their type.
- Package docs should explain concepts and usage patterns; **do not enumerate exports**.

### Testing

- Use `github.com/stretchr/testify/assert` and `require`.
- Table-driven tests with `map[string]struct{}` format.
- Field names: prefer `want` for expected output, `err` for expected errors.
- For inputs, use clear contextual names (e.g., `before`/`after` for diffs, `line`/`col` for positions).
- Always use `t.Parallel()` in all tests.
- Create test packages (`package foo_test`) testing public API.
- Use `require.ErrorIs` for sentinel error checking.
- Use `require.ErrorAs` for error type extraction.
- Use the `go.jacobcolvin.com/x/stringtest` helpers whenever possible.

## Key Dependencies

- `kcl-lang.io/cli` - Upstream KCL CLI (commands wrapped by kclipper)
- `kcl-lang.io/kcl-go` - KCL Go SDK and plugin system
- `helm.sh/helm/v4` - Helm library for chart operations
