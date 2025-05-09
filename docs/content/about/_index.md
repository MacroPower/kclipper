---
title: About
linkTitle: About
description: "About kclipper"
menu: {main: {weight: 20}}
---

[KCL](https://github.com/kcl-lang/kcl) is a **constraint-based record & functional language** mainly used in cloud-native configuration and policy scenarios. It is hosted by the Cloud Native Computing Foundation (CNCF) as a Sandbox Project. The KCL website can be found [here](https://kcl-lang.io/).

Kclipper **combines [KCL](https://github.com/kcl-lang/kcl) and [Helm](https://helm.sh/)**. It is a superset of KCL, and functions by utilizing KCL [plugins](https://www.kcl-lang.io/docs/next/reference/plugin/overview) and wrapping KCL's CLI.

Kclipper has a few primary components:

1. The `helm` KCL [plugin](https://www.kcl-lang.io/docs/next/reference/plugin/overview) to allow Helm to be called directly within KCL codes
2. The `helm` KCL [module](https://www.kcl-lang.io/docs/next/user_docs/concepts/package-and-module) to provide a safe KCL interface to the Helm plugin
3. The `kcl chart` CLI for declarative management of Helm charts and associated JSON/KCL schemas

Together, these enable a powerful and flexible way to manage Helm charts and other KCL resources.
