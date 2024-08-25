# go_template

A template for my Go projects.

## Features

- Configured linter (using [golangci][golangci]).
- Development dependency handling (using [bingo][bingo]).
- Build and release automation (using [goreleaser][goreleaser]).
- Benchmarks (using [benchstat][benchstat] and [benchdiff][benchdiff]).
- Command-line and environment variable parser (using [kong][kong]).
- Leveled logger with logfmt and json support (using [go-kit/log][go-kit-log]).
- Makefile with help, format, lint, test, bench, and build targets.
- GitHub Actions for all of the above.

[golangci]: https://golangci-lint.run
[bingo]: https://github.com/bwplotka/bingo
[goreleaser]: https://goreleaser.com
[benchstat]: https://pkg.go.dev/golang.org/x/perf/cmd/benchstat
[benchdiff]: https://github.com/WillAbides/benchdiff
[kong]: https://github.com/alecthomas/kong
[go-kit-log]: https://github.com/go-kit/log
