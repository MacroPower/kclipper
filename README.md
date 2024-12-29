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
  if regex.match(resource.metadata.name, "^example-.*$"):
    resource.metadata.annotations |= {"example.com/added" = "by kcl patch"}
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
    chart = "podinfo"
    repoURL = "https://stefanprodan.github.io/podinfo"
    targetRevision = "6.7.0"
    valueFiles = [
        "values.yaml",
        "values-prod.yaml"
    ]
    values = {
        replicas = 3
    }
    postRenderer = lambda resource: {str:} -> {str:} {
        if regex.match(resource.metadata.name, "^podinfo-service-test-.*$"):
            resource.metadata.annotations |= {"example.com/added" = "by kcl patch"}
        resource
    }
})
```

> :warning: This must be completed AFTER installing `macropower/kclx`. Just adding the helm module will not provide you with the underlying plugin, and you will get an error when you call the template function.

### `kcl chart` Command

A new `kcl chart` command is available to help you manage Helm charts. Using this command is optional, but again is a huge quality of life improvement that builds on the functionality of a combined helm plugin and package. This command can be used to:

- Bootstrap the Helm package.
- Create a `helm.Chart` schema.
- Generate and manage a JSON Schema for the `helm.Chart` schema's `valueFiles`.
- Manage a KCL `Values` schema for the `helm.Chart` schema's `values`.
- Select the best fit from several different chart schema importers or generators.
- Update all JSON and KCL schemas when a Helm chart is updated.

To achieve this, a `charts.k` file is created in your project directory, which manages configuration for one or more Helm charts. This file can be edited manually, or via the `kcl chart add` command. Entries in `charts.k` are used to inform `kcl chart update`, which is responsible for generating and updating all of the subsequent schemas. You can have a single, global charts.k file, or you can also have multiple charts.k files in different directories (e.g. one per tenant, AppProject, or Application).

For example, we can add a chart to our project:

```bash
kcl chart add -c podinfo -r https://stefanprodan.github.io/podinfo -t "6.7.0"
```

This will create a `charts.k` file with the following contents:

```py
import helm

charts: helm.Charts = {
    podinfo: {
        chart = "podinfo"
        repoURL = "https://stefanprodan.github.io/podinfo"
        targetRevision = "6.7.0"
        schemaGenerator = "AUTO"
    }
}
```

Now we can use `podinfo.Chart` and `podinfo.Values` in our Helm template:

```py
import helm
import charts.podinfo

helm.template(podinfo.Chart {
    values = podinfo.Values {
        replicas = 3
    }
})
```

Going forward, editing the `charts.k` file and running `kcl chart update` will update the `podinfo.Chart` and `podinfo.Values` schemas. E.g., if we set `targetRevision = "6.8.0"` in the charts.k example above, running `kcl chart update` would update the `podinfo.Chart` schema to reflect the new version of the Helm chart, and it would update the `podinfo.Values` schema with any schema changes that have been made between the two revisions.

Note that you can very easily update the `charts.k` file via [KCL Automation](https://www.kcl-lang.io/docs/user_docs/guides/automation). A Renovate config is also coming soon.

### HTTP Plugin

> If needed, this plugin can be disabled with `KCLX_HTTP_PLUGIN_DISABLED=true`.

Alternative HTTP plugin to [kcl-lang/kcl-plugin](https://github.com/kcl-lang/kcl-plugin), which can be used to GET external resources. This one uses plain `net/http`. E.g.:

`http.get("https://example.com", timeout="10s")` -> `{"body": "<...>", "status": 200}`

You can parse the body using one of KCL's native functions e.g. `json.decode` or `yaml.decode`.

### OS Plugin

> If needed, this plugin can be disabled with `KCLX_OS_PLUGIN_DISABLED=true`.

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
