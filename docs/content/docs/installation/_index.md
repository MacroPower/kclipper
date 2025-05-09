---
title: Installation
description: "Install kclipper"
---

{{% pageinfo %}}
This page describes how to install kclipper under the name `kcl`, which allows it to function as a drop-in replacement for KCL.
{{% /pageinfo %}}

OCI artifacts for KCL are available under [packages](https://github.com/MacroPower/kclipper/pkgs/container/kclipper).

Versions are tagged independently of upstream KCL, e.g. kclipper `v0.1.0` maps to kcl `v0.11.0`, but kclipper releases still follow semver with consideration for upstream KCL changes.

### Using Homebrew

> You cannot have both `kcl` and `kclipper` installed via Homebrew. If you already installed `kcl` via Homebrew, you should uninstall it before proceeding.

```bash
brew tap macropower/tap
brew install macropower/tap/kclipper
kcl version
```

If you need the kcl-lsp (e.g. for VSCode), use the official tap:

```bash
brew tap kcl-lang/tap
brew install kcl-lsp
```

The kcl-lsp in this case will call the kclipper binary.

### Using Docker

Docker images are available under [packages](https://github.com/MacroPower/kclipper/pkgs/container/kclipper), e.g.:

```bash
docker pull ghcr.io/macropower/kclipper:latest
```

### Using Release Archives

Binary archives are posted in [releases](https://github.com/MacroPower/kclipper/releases).
