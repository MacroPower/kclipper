package main

import (
	"context"

	"dagger/kclipper/internal/dagger"
)

// benchSuite builds the benchmark suite for kclipper, registering each
// pipeline stage with the shared [Bench] toolchain module. Stages whose base
// requires construction (e.g. the release container) are resolved here so
// errors surface before the suite runs.
func (m *Kclipper) benchSuite(ctx context.Context) (*dagger.Bench, error) {
	releaser, err := m.releaserBase(ctx)
	if err != nil {
		return nil, err
	}

	prettierPatterns, err := m.Prettier.Patterns(ctx)
	if err != nil {
		return nil, err
	}

	return dag.Bench().
		WithStage("env", m.Go.CacheBust(m.Go.Env(dagger.GoEnvOpts{}))).
		WithStage("lint", m.Go.CacheBust(m.Go.LintBase()).
			WithExec([]string{"golangci-lint", "run"})).
		WithStage("test", m.Go.CacheBust(m.Go.Env(dagger.GoEnvOpts{})).
			WithExec([]string{"go", "test", "./..."})).
		WithStage("lint-prettier", m.Go.CacheBust(m.Prettier.LintBase()).
			WithExec(append([]string{"prettier", "--config", "./.prettierrc.yaml", "--check"}, prettierPatterns...))).
		WithStage("lint-actions", m.Go.CacheBust(m.Zizmor.LintBase()).
			WithExec([]string{
				"zizmor", ".github/workflows", "--config", ".github/zizmor.yaml",
			})).
		WithStage("lint-releaser", m.Go.CacheBust(m.Goreleaser.CheckBase()).
			WithExec([]string{"goreleaser", "check"})).
		WithStage("build", m.Go.CacheBust(releaser).
			WithExec([]string{
				"goreleaser", "release", "--snapshot", "--clean",
				"--skip=docker,homebrew,nix,sign,sbom",
				"--parallelism=0",
			})), nil
}

// BenchmarkSummary measures the wall-clock time of key pipeline stages
// and returns a human-readable table. This delegates to the shared [Bench]
// toolchain module for CLI use without jq post-processing.
//
// When parallel is true, all stages run concurrently to measure the
// real-world wall-clock time of the full CI pipeline. The total row
// shows overall elapsed time rather than the sum of individual stages.
//
// +cache="session"
func (m *Kclipper) BenchmarkSummary(
	ctx context.Context,
	// Run stages concurrently to measure full-pipeline wall-clock time.
	// +default=false
	parallel bool,
) (string, error) {
	suite, err := m.benchSuite(ctx)
	if err != nil {
		return "", err
	}
	return suite.Summary(ctx, dagger.BenchSummaryOpts{Parallel: parallel})
}
