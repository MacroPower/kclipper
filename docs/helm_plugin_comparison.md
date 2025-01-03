# Comparison

## To the Helm KCL Plugin

<https://github.com/kcl-lang/helm-kcl>

Don't let the names fool you. The [helm-kcl](https://github.com/kcl-lang/helm-kcl) plugin is a plugin for Helm, allowing you to use `KCLRun` resources in your Helm Charts. The kclipper Helm plugin is a plugin for KCL, allowing you to template Helm Charts in your KCL code. i.e., they integrate in inverse directions.

## To the KCFoil Helm Plugin

<https://github.com/cakehappens/kcfoil>

The kclipper Helm plugin is similar to [kcfoil](https://github.com/cakehappens/kcfoil)'s helm plugin. kcfoil's Helm plugin is based on Tanka's Helm implementation, whereas kclipper's Helm plugin is based on Argo CD's Helm source implementation. So they both expose a helm template function, but the exposed parameters and backend implementations are completely different.

The biggest difference:

- Tanka and ergo kcfoil's plugin expect Helm Charts to be found inside the bounds of a project. i.e., you must "vendor" your Charts, or in other words, you must put your Charts somewhere adjacent to your KCL codes so that it can be referred to using a relative path. This has many advantages but may be cumbersome in some cases.
  - e.g., `helm.template("example", "./charts/example")`
- Argo CD's Helm source implementation, and ergo this plugin as well, allows you to specify a URL to a Helm Chart index, which is useful for fetching Charts from the internet, and it is more heavily optimized for caching fetched results as well. Though, it will likely always be slower versus a vendoring implementation.
  - e.g., `helm.template("example", "0.1.0", "https://example.com/charts")`

Both plugins mirror many aspects of Tanka and Argo CD respectively, including in their overall style, argument usage, and so on. So, the interfaces will feel familiar to users of either tool. I recommend you choose the one that is more familiar to you, and/or best fits your use case.
