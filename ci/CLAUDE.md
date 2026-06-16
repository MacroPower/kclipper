# ci

This repository's own CI module, registered as `ci` in the root `dagger.json`.
It is not designed for remote consumption: it orchestrates the repo's
`dagger -> devbox -> task` flow so CI reproduces exactly what `task check:all`
runs locally, and it owns kclipper's intricate release pipeline.

## Functions

### Checks (run via devbox)

- `lint`, `test` (both +check) run the matching Taskfile target inside the
  project's devbox environment via the `devbox` toolchain, with the Go
  module/build and golangci-lint caches mounted. Tests build with cgo and the
  `netgo` tag (kclipper imports the KCL Go SDK, which needs cgo) so CI exercises
  the same build the release ships.
- `test-coverage` runs the coverage target the same way and returns the coverage
  profile file.
- `lint-renovate` (+check) validates the Renovate configuration with
  renovate-config-validator at a pinned version in a Node container — the one
  gate that runs through neither devbox nor a shared toolchain, so Renovate can
  bump its own validator.

### Lint actions & Security (compose sibling toolchains)

These gates compose a sibling toolchain directly rather than running through
devbox, because their tools are not on the devbox PATH — the same pattern the
release functions use for `goreleaser`.

- `lint-actions` (+check) lints the GitHub Actions workflows by composing the
  `zizmor` toolchain. It pins `.github/zizmor.yaml` as the config path.
- `security` (+check) scans source dependencies for known vulnerabilities by
  composing the `security` toolchain (Trivy). `security-source-sarif` and
  `security-image-sarif` are the non-gating counterparts that emit SARIF for
  GitHub Code Scanning; the image SARIF builds the runtime image the way a
  release publishes it and surfaces OS-layer CVEs the source scan cannot see.
- `lint-releaser` (+check) runs `goreleaser check` via the `goreleaser`
  toolchain.
- `lint-kclmodules` (+check) packages every KCL module under `modules/` (with a
  placeholder version) using the freshly built `kcl` binary, without pushing.

### Release pipeline (composes the goreleaser toolchain; see `build.go` + `publish.go`)

The release pipeline is the intricate part of this module. It cross-compiles a
cgo binary for linux/darwin × amd64/arm64 using a Zig toolchain, a
pre-downloaded KCL language server per platform, and a macOS SDK fetched from
the NixOS binary cache, then bundles and notarizes the macOS binaries and
publishes the KCL modules. It composes the `goreleaser` toolchain directly,
which carries the folded-in cosign signing and syft SBOM tooling
(`with-cosign`/`with-syft`/`sign-keyless`), so the pipeline depends on it alone
for build, sign, and SBOM.

- `releaserBase` (private, in `build.go`) builds the full release container: the
  goreleaser Go base + cosign + syft, then Zig (symlinked onto PATH), the four
  KCL language-server binaries at `/lsp/<os>/<arch>/`, the macOS SDK mounted at
  `/sdk/MacOSX.sdk` (substituted from `cache.nixos.org`), and the `CC_*`/`CXX_*`
  cross-compiler env vars pointing at `hack/zig-*-wrapper.sh`. `EnsureGitRepo`
  (now on the goreleaser toolchain) bootstraps a git repo for GoReleaser.
- `build` / `binary-snapshot` snapshot-cross-compile (no publishing).
  `binary` (and the test-only path) routes through `binary-snapshot`.
- `release --tag=vX.Y.Z` runs GoReleaser for binaries/archives/SBOMs/signing
  (Docker always skipped — images are published natively via Dagger), creates
  the GitHub release, publishes the multi-arch image, and signs digests with
  cosign keyless signing. macOS notarization secrets are plumbed through to
  GoReleaser's `notarize` section. Signing is keyless (Sigstore Fulcio + Rekor):
  the workflow forwards the GitHub Actions OIDC token; with no token the release
  is unsigned.
- `publish-kclmodules --tag=vX.Y.Z` pushes every KCL module under `modules/` to
  the OCI registry (`ghcr.io/macropower/kclipper` by default), skipping
  pre-releases.

Homebrew cask and Nix package handling stays in `.goreleaser.yaml`.

## Layout

- `main.go` defines the `Ci` module (Go module path `dagger/ci`), the check
  functions that run via devbox, the version constants, and `Binary`.
- `check.go` holds the remaining `+check` functions and the manual
  `release-dry-run` (build + verify-binary-platform + image build).
- `build.go` holds `Build`/`BinarySnapshot`/`BuildImages`, the runtime image
  builders, `releaserBase`, and the Zig/LSP/macOS-SDK helpers.
- `publish.go` holds `Release`/`PublishImages`/`PublishKCLModules` and the
  image publish/sign helpers.
- Dependencies in `dagger.json`: the `devbox` toolchain (checks), the
  `goreleaser` toolchain (release, carrying cosign + syft), the `security`
  toolchain (the vulnerability scan), and the `zizmor` toolchain (the Actions
  workflow lint), all referenced remotely from `github.com/MacroPower/x`.
- The `tests/` submodule exercises the +check functions (build dist, image
  metadata, lint-releaser, binary, lint-actions, lint-kclmodules); the
  network-dependent `test-publish-images` is non-`+check` and run manually.

The `engineVersion` in `dagger.json` is pinned in lockstep with the root
`dagger.json` and with the CLI version in `.github/workflows`; bump them together
via `task dagger:update VERSION=<tag>`.

## Version Management

Tool versions are Go constants at the top of `main.go` with Renovate
annotations (`// renovate: datasource=... depName=...`): goreleaser, Zig, the
KCL language server, the nix/debian/node base images, and the macOS SDK store
path. The Renovate `customManagers` in `.github/renovate.json5` track these (and
the Taskfile-pinned tool versions); devbox.json tool versions are handled by
Renovate's built-in devbox manager.
