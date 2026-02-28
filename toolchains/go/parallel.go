package main

import (
	"context"

	"github.com/sourcegraph/conc/pool"
	"go.opentelemetry.io/otel"
)

// parallelJob pairs a name with a function to execute.
type parallelJob struct {
	name string
	fn   func(ctx context.Context) error
}

// parallelJobs collects jobs and runs them concurrently with bounded
// parallelism and per-job OTEL spans for Dagger TUI visibility.
type parallelJobs struct {
	jobs  []parallelJob
	limit int
}

// newParallel creates a new [parallelJobs] builder.
func newParallel() parallelJobs {
	return parallelJobs{}
}

// withJob adds a named job. Uses a value receiver to support a clone/builder
// pattern.
func (p parallelJobs) withJob(name string, fn func(ctx context.Context) error) parallelJobs {
	p.jobs = append(append([]parallelJob{}, p.jobs...), parallelJob{name: name, fn: fn})
	return p
}

// withLimit sets the maximum number of concurrent goroutines.
func (p parallelJobs) withLimit(n int) parallelJobs {
	p.limit = n
	return p
}

// run executes all jobs concurrently, creating an OTEL span per job.
func (p parallelJobs) run(ctx context.Context) error {
	pl := pool.New().WithErrors().WithContext(ctx)
	if p.limit > 0 {
		pl = pl.WithMaxGoroutines(p.limit)
	}
	for _, j := range p.jobs {
		pl.Go(func(ctx context.Context) error {
			ctx, span := otel.Tracer("dagger").Start(ctx, j.name)
			defer span.End()
			return j.fn(ctx)
		})
	}
	return pl.Wait()
}
