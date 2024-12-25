# macropower/kclx

[KCL](https://github.com/kcl-lang/kcl) is a constraint-based record & functional language mainly used in cloud-native configuration and policy scenarios. It is hosted by the Cloud Native Computing Foundation (CNCF) as a Sandbox Project. The KCL website can be found [here](https://kcl-lang.io/).

kclx = KCL Extended. This repo includes an opinionated set of extensions for KCL (thus macropower/kclx, other flavors are available). The included extensions are primarily centered upon improving the experience of using KCL with [Argo CD](https://argoproj.github.io/cd/), though they are not necessarily limited to that. In the context of this repo, "extensions" is meant to refer to a set of both KCL [plugins](https://www.kcl-lang.io/docs/next/reference/plugin/overview) and [packages](https://www.kcl-lang.io/docs/next/user_docs/concepts/package-and-module).

To use macropower/kclx, you must [install](#installation) it as a KCL replacement. The macropower/kclx binary will wrap the upstream KCL release with its plugins. Up-to-date multi-architecture Docker images for x86 and arm64 are also available.

> :warning: You should not currently use macropower/kclx in multi-tenant Argo CD environments. See [#2](https://github.com/MacroPower/kclx/issues/2).

## Extensions

### Helm Plugin

Execute `helm template` and return the resulting Kubernetes resources. This plugin uses Argo CD's Helm source implementation on the backend, and is very fast once the upstream chart has been cached (<100ms even on my older arm-based system). E.g.:

```py
helm.template(
    chart="example",
    target_revision="0.1.0",
    repo_url="https://example.com/charts",
) # -> [{"group": "apps", "kind": "Deployment", ...}, ...]
```

You can then use these resources in your KCL code (e.g., via merging in some changes, referencing the resources elsewhere, etc.). You can also very flexibly patch the templated resources with a lambda function. E.g.:

```py
import regex
import kcl_plugin.helm

_chart = helm.template(
  chart="example",
  target_revision="0.1.0",
  repo_url="https://example.com/charts",
  values={
    replicas = 3
  }
)

patch = lambda resource: {str:} -> {str:} {
  if resource.kind == "Deployment":
    resource.spec.strategy.type = "RollingUpdate"

  if regex.match(resource.metadata.name, "^example-.*$"):
    resource.metadata.annotations: {
      "example.com/added" = "by kcl patch"
    }

  resource
}

{"resources": [patch(r) for r in _chart]}
```

To read more about how the macropower/kclx Helm plugin compares to other KCL Helm plugins like [kcfoil](https://github.com/cakehappens/kcfoil), see the [Helm plugin comparison](docs/helm_plugin_comparison.md).

### Helm Package

To gain full support for the Helm plugin with kcl-language-server (for highlighting, completion, and definitions in your editor), you can use the `macropower/kclx/helm` wrapper package. This is completely optional, but is a significant quality of life improvement.

```sh
kcl mod add oci://ghcr.io/macropower/kclx/helm
```

Then, you can use the `helm` package to interface with the Helm plugin, rather than calling it directly:

```py
import helm

helm.template(helm.Chart {
  chart = "example"
  targetRevision = "0.1.0"
  repoURL = "https://example.com/charts"
  values = {
    replicas = 3
  }
})
```

> :warning: This must be completed AFTER installing `macropower/kclx`. Just adding the helm module will not provide you with the underlying plugin, and you will get an error when you call the template function.

### Helm Schema Command

You can also use a schema for the `values` argument. This schema can be imported from a Helm Chart's `values.schema.json` file if one is available, or alternatively it can be generated from one or more `values.yaml` files.

**TODO**:

```bash
# Generate KCL models from chart values
kcl import -m helmschema values.yaml
```

For now, a [workaround](docs/helm_values_schema.md) is available which requires a few manual steps. The end result is you'll have a `values.schema.k` file that you can use in your KCL code, which exports a root schema called `Values`. E.g.:

```py
import helm

helm.template(helm.Chart {
  chart = "example"
  targetRevision = "0.1.0"
  repoURL = "https://example.com/charts"
  values = Values { # <- Uses the Values schema from values.schema.k
    replicas = 3
  }
})
```

### HTTP Plugin

Alternative HTTP plugin to [kcl-lang/kcl-plugin](https://github.com/kcl-lang/kcl-plugin), which can be used to GET external resources. This one uses plain `net/http`. E.g.:

`http.get("https://example.com")` -> `{"body": "<...>", "status": 200}`

You can parse the body using one of KCL's native functions e.g. `json.decode` or `yaml.decode`.

### OS Plugin

Run a command on the host OS. This can be useful for integrating with other tools that do not have a native KCL plugin available, e.g. by installing them in your container. E.g.:

`os.exec("command", ["arg"])` -> `{"stdout": "x", "stderr": "y"}`

You can parse stdout using one of KCL's native functions e.g. `json.decode` or `yaml.decode`.

## Installation

Binaries are posted in [releases](https://github.com/MacroPower/kclx/releases). Images and OCI artifacts are available under [packages](https://github.com/MacroPower/kclx/pkgs/container/kclx).

The binary name for macropower/kclx is always still just `kcl`, so that it can be used as a drop-in replacement for official KCL binaries. Versions are tagged independently of upstream KCL, e.g. macropower/kclx `v0.1.0` maps to kcl `v0.11.0`, but macropower/kclx releases still follow semver with consideration for upstream KCL changes.

To use macropower/kclx with Argo CD, you can follow [this guide](https://www.kcl-lang.io/docs/user_docs/guides/gitops/gitops-quick-start) to set up the KCL ConfigManagementPlugin. You just need to substitute the official kcl image with a macropower/kclx image.

## Contributing

[Tasks](https://taskfile.dev) are available (run `task help`).

If you are using an arm64 Mac, you can use [Devbox](https://www.jetify.com/docs/devbox/) to create a Nix environment pre-configured with all the necessary tools and dependencies for Go, Zig, etc. Otherwise, you can still use the included Devbox, but CGO probably won't work.

## License

KCL and this project are both licensed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.

KCL is copyright The KCL Authors, all rights reserved.
