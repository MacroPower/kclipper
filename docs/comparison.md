# Comparison

## To Holos

<https://holos.run>

Holos is similar to kclipper, however Holos is built on CUE, whereas kclipper is built on KCL.

In general, CUE and Holos, as well as KCL and kclipper, both are used for similar purposes. However, differences in philosophy and goals have led to very different implementations. I would recommend reading the following links for a more in-depth explanation from the creators of both languages:

- <https://www.kcl-lang.io/docs/0.10/user_docs/getting-started/intro#vs-cue>
- <https://cuelang.org/docs/concept/configuration-use-case/#comparisons>

The TL;DR:

CUE's focus is data validation. Traditionally, languages like CUE, e.g. Jsonnet, focused on data templating (boilerplate removal). KCL attempts to fully capture both use cases, though this comes at the cost of it being a bit closer to a general-purpose programming language, ergo inviting more complexity, which may not be desirable in all scenarios.

- KCL is Python-like, whereas CUE is JSON-like (and is a superset of JSON).
- KCL is statically compiled, whereas CUE performs all constraint checks at runtime.
  - Holos focuses on implementing the [rendered manifest pattern](https://akuity.io/blog/the-rendered-manifests-pattern) to generate Kubernetes manifests, which somewhat eliminates performance bottlenecks as a concern. However, this strategy may not be suitable for everyone.
- KCL has many object-oriented features, such as single inheritance, methods, and mutation of private fields.
- KCL is incredibly flexible, CUE takes an approach of having "restrictions [that] reduce flexibility, but also enhance clarity".

These overarching differences have different implications for tools like Holos and kclipper. For example, Holos is a somewhat standalone system providing an opinionated integration layer for your platform, whereas kclipper simply provides tooling necessary to build your own platform management systems in KCL (e.g. on top of existing patterns, like [konfig](https://github.com/kcl-lang/konfig)).

You will likely find that you prefer one over the other based on your personal preferences and use cases.

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
