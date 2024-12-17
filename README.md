# kclx

KCL Extended.

[KCL](https://github.com/kcl-lang/kcl) is an open-source, constraint-based record and functional language that enhances the writing of complex configurations, including those for cloud-native scenarios. The KCL website can be found [here](https://kcl-lang.io/).

KCL Extended serves to wrap upstream KCL releases with additional features and [plugins](https://www.kcl-lang.io/docs/next/reference/plugin/overview), and provide up-to-date multi-architecture Docker images for x86 and arm64.

## Installation

Binaries are posted in [releases](https://github.com/MacroPower/kclx/releases).

Images are available [here](https://github.com/MacroPower/kclx/pkgs/container/kclx). e.g. `ghcr.io/macropower/kclx:latest`

The command and binary is still just `kcl`, so that it can be used as a drop-in replacement for official KCL binaries.

Versions are tagged independently of upstream KCL, e.g. kclx `v0.1.0` maps to kcl `v0.10.10`, but kclx releases still follow semver with consideration for upstream KCL changes. e.g., bumping upstream KCL's major version will bump this project's major version as well. I considered using a version strategy like `v0.1.0-kcl0.10.0`, but decided against it for simplicity and compatibility with other tools (like goreleaser, your renovate config, etc.)

## Included Plugins

- `os`
  - `os.exec("command", ["arg"])` -> `{"stdout": "x", "stderr": "y"}`

## Contributing

[Tasks](https://taskfile.dev) are available (run `task help`).

If you are using an arm64 Mac, you can use [Devbox](https://www.jetify.com/docs/devbox/) to create a Nix environment pre-configured with all the necessary tools and dependencies for Go, Zig, etc. Otherwise, you can still use the included Devbox, but CGO probably won't work.
