# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`kclipper` is a superset of KCL that integrates Helm chart management. It provides KCL plugins, packages, and CLI commands to manage Helm charts declaratively and render them directly within KCL code. The binary is named `kcl` and can be used as a drop-in replacement.

## Build & Development Commands

```bash
# Format code
task format

# Lint all (Go, YAML, Actions, Renovate, GoReleaser)
task lint

# Run tests
task test

# Run a single test
go test ./pkg/helm -run TestHelmChart
```

## Architecture

### Entry Point & CLI

- `cmd/kclipper/main.go` - Entry point, wraps upstream KCL CLI and registers plugins
- `cmd/kclipper/commands/` - Root command setup, adds `chart` and `export` subcommands to upstream KCL commands

### Core Packages (`pkg/`)

**Helm Integration:**

- `pkg/helm/` - Chart templating, pulling, dependency resolution (based on Argo CD's implementation, optimized for minimal I/O)
- `pkg/helmrepo/` - Helm repository management and authentication
- `pkg/helmtest/` - Test utilities for Helm operations

**KCL Plugins:**

- `pkg/kclplugin/helm/` - KCL plugin exposing `helm.template()` to KCL code
- `pkg/kclplugin/filepath/` - KCL plugin for filepath operations (base, dir, ext, join, etc.)
- `pkg/kclplugin/plugins/` - Shared plugin utilities (SafeMethodArgs for safe argument access)

**KCL Module Generation:**

- `pkg/kclmodule/kclchart/` - Chart struct definition and KCL schema generation for per-chart packages
- `pkg/kclmodule/kclhelm/` - Helm Chart struct (postRenderer, valueFiles) and base schema generation

**Schema Generation:**

- `pkg/jsonschema/` - Schema generators for Helm values (AUTO, VALUE-INFERENCE, URL, CHART-PATH, LOCAL-PATH)
- `pkg/crd/` - CRD schema generators (TEMPLATE, CHART-PATH, PATH)
- `pkg/kclgen/` - Generates KCL schemas from JSON Schemas and CRDs

**Chart Management:**

- `pkg/chartcmd/` - Logic for `kcl chart` subcommands (add, update, set, init, repo add)
- `pkg/charttui/` - Interactive TUI for chart operations

**Other:**

- `pkg/paths/` - Path resolution for charts and modules
- `pkg/kube/` - Kubernetes YAML utilities
- `pkg/kclexport/` - Export KCL content to other formats
- `pkg/kclerrors/` - Standardized error types and handling
- `pkg/kclautomation/` - KCL file automation (e.g. for chart declarations)
- `pkg/syncs/` - Synchronization primitives (key locks for concurrency)
- `pkg/version/` - Version information management
- `pkg/log/` - Logging management

### KCL Modules (`modules/`)

- `helm/` - Published KCL module with `Charts`, `ChartRepos`, and related schemas
- `filepath/` - File path utilities for KCL

### Data Flow

1. User declares charts in `charts/charts.k` using `helm.Charts` schema
2. `kcl chart update` generates per-chart packages with `Chart` and `Values` schemas
3. User code imports chart packages and calls `helm.template(chart.Chart{...})`
4. Plugin renders chart at runtime, returns list of Kubernetes resources
5. Output piped to kubectl, GitOps workflow, or Argo CD CMP

## Code Style

### Go Conventions

- Document all exported items with doc comments
- Use `[Name]` syntax for Go doc links
- Package documentation in `doc.go` files
- Wrap errors with `fmt.Errorf("context: %w", err)` - no "failed" or "error" in messages
- Use global error variables for common errors

### Testing

- Use `github.com/stretchr/testify/assert` and `require`
- Table-driven tests with `map[string]struct{}` format
- Field names: `input`, `want`, `got`, `err`
- Always use `t.Parallel()` in all tests
- Create test packages (`package foo_test`) testing public API
- Use `require.ErrorIs` for error type checking

## Key Dependencies

- `kcl-lang.io/cli` - Upstream KCL CLI (commands wrapped by kclipper)
- `kcl-lang.io/kcl-go` - KCL Go SDK and plugin system
- `helm.sh/helm/v3` - Helm library for chart operations
- `github.com/charmbracelet/bubbletea` - TUI framework for interactive chart management
- `github.com/dadav/helm-schema` - Schema inference from values.yaml
