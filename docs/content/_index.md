---
title: kclipper
---

{{< blocks/cover title="KCL + Helm = kclipper" image_anchor="top" height="full" >}}
<a class="btn btn-lg btn-primary me-3 mb-4" href="/about/">
  Learn More
</a>
<a class="btn btn-lg btn-secondary me-3 mb-4" href="/docs/">
  Get Started <i class="fas fa-arrow-alt-circle-right ms-2 "></i>
</a>
{{< blocks/link-down color="info" >}}
{{< /blocks/cover >}}


{{% blocks/lead color="primary" %}}
[KCL](https://github.com/kcl-lang/kcl) is a **constraint-based record & functional language** mainly used in cloud-native configuration and policy scenarios. It is hosted by the Cloud Native Computing Foundation (CNCF) as a Sandbox Project. The KCL website can be found [here](https://kcl-lang.io/).

**Kclipper is a drop-in replacement for KCL**, which **combines [KCL](https://github.com/kcl-lang/kcl) and [Helm](https://helm.sh/)**. It provides [plugins](https://www.kcl-lang.io/docs/next/reference/plugin/overview) to **allow Helm to be called directly within KCL codes**, and [modules](https://www.kcl-lang.io/docs/next/user_docs/concepts/package-and-module) to provide safe KCL interfaces to the KCL plugins. Additionally, it adds extra commands to enable **declarative management of Helm charts and associated JSON/KCL schemas**.

{{% /blocks/lead %}}

{{% blocks/section color="dark" type="row" %}}
{{% blocks/feature icon="fa-circle-check" title="Type Safe" %}}
Create and use KCL schemas for Helm chart values and CRDs. Gain access to inline documentation, auto-completion, and validation -- enjoy a consistent experience across all resources.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-lightbulb" title="Unopinionated" %}}
You can go your own way. Use kclipper with [konfig](https://github.com/kcl-lang/konfig), or build your own platform management system from scratch.
{{% /blocks/feature %}}

{{% blocks/feature icon="fab fa-github" title="Fully Declarative" %}}
Kclipper includes a helpful CLI to manage charts declaratively in KCL, which you can commit to source control to apply directly via Argo CD, or trigger manifest hydration.
{{% /blocks/feature %}}
{{% /blocks/section %}}

{{% blocks/section type="row" %}}
For a full example, see [MacroPower/homelab](https://github.com/MacroPower/homelab).
{.h3 .text-center}
{{% /blocks/section %}}
