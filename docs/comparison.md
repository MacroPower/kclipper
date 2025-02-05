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

## To other KCL plugins

### kcfoil

<https://github.com/cakehappens/kcfoil>

- kcfoil's interface is based on Tanka, whereas kclipper's interface is based on ArgoCD.

kcfoil is overall a more focused and simple plugin, if it meets your needs, it may be a better choice for you.

### knit

<https://github.com/tvandinther/knit>

- knit focuses on implementing the [rendered manifest pattern](https://akuity.io/blog/the-rendered-manifests-pattern), whereas kclipper is more focused on use cases where manifests are rendered directly by a CMP or similar.
- knit provides its own CLI, whereas kclipper wraps/vendors the KCL CLI.
- knit provides interfaces for both Helm and Kustomize, whereas kclipper is focused only on Helm and fully relies on KCL for Kustomize-like features.
- Both knit and kclipper support vendoring KCL schemas for helm charts, but implementations of this are very different.

If you need direct support for Kustomize, are implementing rendered manifest pattern, or simply prefer knit's approach to vendoring/etc., knit may be a better choice for you.
