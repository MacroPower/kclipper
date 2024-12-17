# kclx

KCL Extended.

[KCL](https://github.com/kcl-lang/kcl) is a constraint-based record & functional domain language. Full documents of KCL can be found [here](https://kcl-lang.io/).

This repo tracks upstream KCL releases and provides multi-architecture Docker images for x86 and arm64.

## Installation

Binaries are posted in [releases](https://github.com/MacroPower/kclx/releases).

Images are available [here](https://github.com/MacroPower/kclx/pkgs/container/kclx).

e.g. `ghcr.io/macropower/kclx:latest`

The command and binary is still just `kcl`, so that it can be used as a drop-in replacement for official KCL binaries.

## Included Plugins

- `os`
  - `os.exec("command", ["arg"])`

## Contributing

[Tasks](https://taskfile.dev) are available (run `task help`).

If you are using an arm64 Mac, you can use [Devbox](https://www.jetify.com/docs/devbox/) to create a Nix environment pre-configured with all the necessary tools and dependencies for Go, Zig, etc. Otherwise, you can still use the included Devbox, but CGO probably won't work.
