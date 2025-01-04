# Helm Extensions

## Helm Plugin

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

manifests.yaml_stream(
  [patch(r) for r in _chart]
)
```

To read more about how the kclipper Helm plugin compares to other KCL Helm plugins like [kcfoil](https://github.com/cakehappens/kcfoil), see the [Helm plugin comparison](docs/comparison.md).

## Helm Package

To gain full support for the Helm plugin with kcl-language-server (for highlighting, completion, and definitions in your editor), you can use the `macropower/kclipper/helm` wrapper package. This is completely optional, but is a significant quality of life improvement.

```sh
kcl mod add oci://ghcr.io/macropower/kclipper/helm
```

Then, you can use the `helm` package to interface with the Helm plugin, rather than calling it directly:

```py
import helm
import regex
import manifests

_podinfo = helm.template(helm.Chart {
    chart = "podinfo"
    repoURL = "https://stefanprodan.github.io/podinfo"
    targetRevision = "6.7.0"
    valueFiles = [
        "values.yaml",
        "values-prod.yaml"
    ]
    values = {
        replicaCount = 3
    }
    postRenderer = lambda resource: {str:} -> {str:} {
        if regex.match(resource.metadata.name, "^podinfo-service-test-.*$"):
            resource.metadata.annotations |= {"example.com/added" = "by kcl patch"}
        resource
    }
})

manifests.yaml_stream(_podinfo)
```

> :warning: This must be completed AFTER installing kclipper. Just adding the helm module will not provide you with the underlying plugin, and you will get an error when you call the template function.

## `kcl chart` Command

A new `kcl chart` command is available to help you manage Helm charts. Using this command is optional, but again is a huge quality of life improvement that builds on the functionality of a combined helm plugin and package. This command can be used to:

- Bootstrap the Helm package.
- Create a `helm.Chart` schema with defaults populated.
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

Alternatively, we can load values from a local values.yaml file, using the managed `values.schema.json` file:

```py
import helm
import charts.podinfo

helm.template(podinfo.Chart {
    valueFiles = ["values.yaml"]
})
```

```yaml
# yaml-language-server: $schema=./charts/podinfo/values.schema.json

replicas: 3
```

If you use both `values` and `valueFiles`, note that `values` will always take precedence.

Going forward, editing the `charts.k` file and running `kcl chart update` will update the `podinfo.Chart` and `podinfo.Values` schemas. E.g., if we set `targetRevision = "6.8.0"` in the charts.k example above, running `kcl chart update` would update the `podinfo.Chart` schema to reflect the new version of the Helm chart, and it would update the `podinfo.Values` schema with any schema changes that have been made between the two revisions.

Note that you can very easily update the `charts.k` file via [KCL Automation](https://www.kcl-lang.io/docs/user_docs/guides/automation). A Renovate config is also coming soon.
