package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/kclipper/internal/dagger"

	"golang.org/x/sync/errgroup"
)

// BenchmarkResult holds the timing for a single pipeline stage.
type BenchmarkResult struct {
	// Pipeline stage name (e.g. "env", "lint", "test").
	Name string
	// Duration in seconds.
	DurationSecs float64
	// Whether the stage completed successfully.
	Ok bool
	// Error message if the stage failed.
	Error string
}

// Benchmark measures the wall-clock time of key pipeline stages and
// returns structured results. Use this to identify bottlenecks and track
// performance regressions. Each stage is run sequentially to isolate
// timings.
//
// +cache="session"
func (m *Kclipper) Benchmark(ctx context.Context) ([]*BenchmarkResult, error) {
	return m.runBenchmarks(ctx, false)
}

// BenchmarkSummary measures the wall-clock time of key pipeline stages
// and returns a human-readable table. This is a convenience wrapper
// around [Kclipper.Benchmark] for CLI use without jq post-processing.
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
	results, err := m.runBenchmarks(ctx, parallel)
	if err != nil {
		return "", err
	}
	return formatBenchmarkTable(results, parallel), nil
}

// formatBenchmarkTable formats benchmark results as an aligned text table.
func formatBenchmarkTable(results []*BenchmarkResult, parallel bool) string {
	var b strings.Builder

	mode := "sequential"
	if parallel {
		mode = "parallel"
	}
	fmt.Fprintf(&b, "Benchmark (%s)\n", mode)
	fmt.Fprintf(&b, "%-20s %10s %8s\n", "STAGE", "DURATION", "STATUS")
	fmt.Fprintf(&b, "%-20s %10s %8s\n", "-----", "--------", "------")

	var total float64
	var maxDur float64
	allOk := true
	for _, r := range results {
		status := "ok"
		if !r.Ok {
			status = "FAIL"
			allOk = false
		}
		fmt.Fprintf(&b, "%-20s %9.1fs %8s\n", r.Name, r.DurationSecs, status)
		total += r.DurationSecs
		if r.DurationSecs > maxDur {
			maxDur = r.DurationSecs
		}
	}

	fmt.Fprintf(&b, "%-20s %10s %8s\n", "-----", "--------", "------")

	// In parallel mode, show both the wall-clock (max) and sum of stages.
	totalStatus := "ok"
	if !allOk {
		totalStatus = "FAIL"
	}
	if parallel {
		fmt.Fprintf(&b, "%-20s %9.1fs %8s\n", "WALL-CLOCK", maxDur, totalStatus)
		fmt.Fprintf(&b, "%-20s %9.1fs\n", "SUM", total)
	} else {
		fmt.Fprintf(&b, "%-20s %9.1fs %8s\n", "TOTAL", total, totalStatus)
	}

	return b.String()
}

// benchmarkStage pairs a stage name with its execution function.
type benchmarkStage struct {
	name string
	fn   func(context.Context) error
}

// benchmarkStages returns the list of pipeline stages to benchmark,
// including the kclipper-specific build stage.
func (m *Kclipper) benchmarkStages() []benchmarkStage {
	return []benchmarkStage{
		{"env", func(ctx context.Context) error {
			_, err := m.Go.CacheBust(m.Go.Env(dagger.GoEnvOpts{})).Sync(ctx)
			return err
		}},
		{"lint", func(ctx context.Context) error {
			return m.Go.Lint(ctx)
		}},
		{"test", func(ctx context.Context) error {
			_, err := m.Go.CacheBust(m.Go.Env(dagger.GoEnvOpts{})).
				WithExec([]string{"go", "test", "./..."}).
				Sync(ctx)
			return err
		}},
		{"lint-prettier", func(ctx context.Context) error {
			_, err := m.Go.CacheBust(m.prettierBase()).
				WithMountedDirectory("/src", m.Source).
				WithWorkdir("/src").
				WithExec(append(
					[]string{"prettier", "--config", "./.prettierrc.yaml", "--check"},
					defaultPrettierPatterns()...,
				)).
				Sync(ctx)
			return err
		}},
		{"lint-actions", func(ctx context.Context) error {
			_, err := m.Go.CacheBust(dag.Container().
				From("ghcr.io/zizmorcore/zizmor:"+zizmorVersion)).
				WithMountedDirectory("/src", m.Source).
				WithWorkdir("/src").
				WithExec([]string{
					"zizmor", ".github/workflows", "--config", ".github/zizmor.yaml",
				}).
				Sync(ctx)
			return err
		}},
		{"lint-releaser", func(ctx context.Context) error {
			_, err := m.Go.CacheBust(m.goreleaserCheckBase(kclipperCloneURL)).
				WithExec([]string{"goreleaser", "check"}).
				Sync(ctx)
			return err
		}},
		{"build", func(ctx context.Context) error {
			_, err := m.Go.CacheBust(m.releaserBase()).
				WithExec([]string{
					"goreleaser", "release", "--snapshot", "--clean",
					"--skip=docker,homebrew,nix,sign,sbom",
					"--parallelism=0",
				}).
				Sync(ctx)
			return err
		}},
	}
}

// runBenchmarks executes benchmark stages. When parallel is false, stages
// run sequentially for isolated timings. When true, stages run concurrently
// to measure real-world wall-clock time.
func (m *Kclipper) runBenchmarks(ctx context.Context, parallel bool) ([]*BenchmarkResult, error) {
	stages := m.benchmarkStages()

	if parallel {
		return m.runBenchmarksParallel(ctx, stages)
	}
	return m.runBenchmarksSequential(ctx, stages)
}

// runBenchmarksSequential runs each stage one at a time for isolated timings.
func (m *Kclipper) runBenchmarksSequential(ctx context.Context, stages []benchmarkStage) ([]*BenchmarkResult, error) {
	results := make([]*BenchmarkResult, 0, len(stages))
	for _, s := range stages {
		start := time.Now()
		err := s.fn(ctx)
		elapsed := time.Since(start).Seconds()

		r := &BenchmarkResult{
			Name:         s.name,
			DurationSecs: elapsed,
			Ok:           err == nil,
		}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	return results, nil
}

// runBenchmarksParallel runs all stages concurrently and reports individual
// wall-clock times. This measures what a real CI run looks like when Dagger
// evaluates pipelines in parallel.
func (m *Kclipper) runBenchmarksParallel(ctx context.Context, stages []benchmarkStage) ([]*BenchmarkResult, error) {
	results := make([]*BenchmarkResult, len(stages))
	g, gCtx := errgroup.WithContext(ctx)

	for i, s := range stages {
		g.Go(func() error {
			start := time.Now()
			err := s.fn(gCtx)
			elapsed := time.Since(start).Seconds()

			r := &BenchmarkResult{
				Name:         s.name,
				DurationSecs: elapsed,
				Ok:           err == nil,
			}
			if err != nil {
				r.Error = err.Error()
			}
			results[i] = r
			return nil // always collect results, don't abort on stage failure
		})
	}

	_ = g.Wait()
	return results, nil
}
