# kclipper/helm

```sh
kcl mod add oci://ghcr.io/macropower/kclipper/helm
```

## Index

- [Chart](#chart)
- [ChartConfig](#chartconfig)
- [ChartRepo](#chartrepo)
- [ValueInferenceConfig](#valueinferenceconfig)
- [Resource](#resource)

## Schemas

### Chart

Defines a Helm chart.

#### Attributes

| name                   | type                                             | description                                                                                                  | default value |
| ---------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------ | ------------- |
| **chart** `required`   | str                                              | Helm chart name.                                                                                             |               |
| **namespace**          | str                                              | Optional namespace to template with.                                                                         |               |
| **passCredentials**    | bool                                             | Set to `True` to pass credentials to all domains (Helm's `--pass-credentials`).                              |               |
| **postRenderer**       | ([Resource](#resource)) -> [Resource](#resource) | Lambda function to modify the Helm template output. Evaluated for each resource in the Helm template output. |               |
| **releaseName**        | str                                              | Helm release name to use. If omitted the chart name will be used.                                            |               |
| **repoURL** `required` | str                                              | URL of the Helm chart repository.                                                                            |               |
| **repositories**       | [[ChartRepo](#chartrepo)]                        | Helm chart repositories.                                                                                     |               |
| **schemaValidator**    | "KCL" \| "HELM"                                  | Validator to use for the Values schema.                                                                      |               |
| **skipCRDs**           | bool                                             | Set to `True` to skip the custom resource definition installation step (Helm's `--skip-crds`).               |               |
| **skipHooks**          | bool                                             | Set to `True` to skip templating Helm hooks (similar to Helm's `--no-hooks`).                                |               |
| **targetRevision**     | str                                              | Semver tag for the chart's version. May be omitted for local charts.                                         |               |
| **valueFiles**         | [str]                                            | Helm value files to be passed to Helm template.                                                              |               |
| **values**             | any                                              | Helm values to be passed to Helm template. These take precedence over valueFiles.                            |               |

### ChartConfig

Configuration that can be defined in `charts.k`, in addition to those specified in `helm.ChartBase`.

#### Attributes

| name                   | type                                                                           | description                                                                                                       | default value |
| ---------------------- | ------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------- | ------------- |
| **chart** `required`   | str                                                                            | Helm chart name.                                                                                                  |               |
| **crdPaths**           | [str]                                                                          | Paths to any CRDs to import as schemas. Can be file and/or URL paths. Glob patterns are supported.                |               |
| **namespace**          | str                                                                            | Optional namespace to template with.                                                                              |               |
| **passCredentials**    | bool                                                                           | Set to `True` to pass credentials to all domains (Helm's `--pass-credentials`).                                   |               |
| **releaseName**        | str                                                                            | Helm release name to use. If omitted the chart name will be used.                                                 |               |
| **repoURL** `required` | str                                                                            | URL of the Helm chart repository.                                                                                 |               |
| **repositories**       | [[ChartRepo](#chartrepo)]                                                      | Helm chart repositories.                                                                                          |               |
| **schemaGenerator**    | "AUTO" \| "VALUE-INFERENCE" \| "URL" \| "CHART-PATH" \| "LOCAL-PATH" \| "NONE" | Schema generator to use for the Values schema.                                                                    |               |
| **schemaPath**         | str                                                                            | Path to the schema to use, when relevant for the selected schemaGenerator.                                        |               |
| **schemaValidator**    | "KCL" \| "HELM"                                                                | Validator to use for the Values schema.                                                                           |               |
| **skipCRDs**           | bool                                                                           | Set to `True` to skip the custom resource definition installation step (Helm's `--skip-crds`).                    |               |
| **skipHooks**          | bool                                                                           | Set to `True` to skip templating Helm hooks (similar to Helm's `--no-hooks`).                                     |               |
| **targetRevision**     | str                                                                            | Semver tag for the chart's version. May be omitted for local charts.                                              |               |
| **valueInference**     | [ValueInferenceConfig](#valueinferenceconfig)                                  | Configuration for value inference via dadav/helm-schema. Requires schemaGenerator to be set to `VALUE-INFERENCE`. |               |

### ChartRepo

Defines a Helm chart repository.

#### Attributes

| name                      | type | description                                                                                               | default value |
| ------------------------- | ---- | --------------------------------------------------------------------------------------------------------- | ------------- |
| **caPath**                | str  | CA file path.                                                                                             |               |
| **insecureSkipVerify**    | bool | Set to `True` to skip SSL certificate verification.                                                       |               |
| **name** `required`       | str  | Helm chart repository name for reference by `@name`.                                                      |               |
| **passCredentials**       | bool | Set to `True` to allow credentials to be used in chart dependencies defined by charts in this repository. |               |
| **passwordEnv**           | str  | Basic authentication password environment variable.                                                       |               |
| **tlsClientCertDataPath** | str  | TLS client certificate data path.                                                                         |               |
| **tlsClientCertKeyPath**  | str  | TLS client certificate key path.                                                                          |               |
| **url** `required`        | str  | Helm chart repository URL.                                                                                |               |
| **usernameEnv**           | str  | Basic authentication username environment variable.                                                       |               |

### Resource

Kubernetes resource.

#### Attributes

| name                      | type                      | description                                                                                                                                             | default value |
| ------------------------- | ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| **apiVersion** `required` | str                       | Identifies the version of the object's schema. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |               |
| **kind** `required`       | str                       | Identifies the object's schema. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds              |               |
| **metadata** `required`   | [ObjectMeta](#objectmeta) | Describes the object's metadata. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata                |               |

### ValueInferenceConfig

ValueInferenceConfig defines configuration for value inference via dadav/helm-schema.

#### Attributes

| name                          | type | description                                                  | default value |
| ----------------------------- | ---- | ------------------------------------------------------------ | ------------- |
| **helmDocsCompatibilityMode** | bool | Parse and use helm-docs comments.                            |               |
| **keepFullComment**           | bool | Keep the whole leading comment (default: cut at empty line). |               |
| **keepHelmDocsPrefix**        | bool | Keep the helm-docs prefix (--) in the schema.                |               |
| **removeGlobal**              | bool | Remove the global key from the schema.                       |               |
| **skipAdditionalProperties**  | bool | Skip auto-generation of AdditionalProperties.                |               |
| **skipDefault**               | bool | Skip auto-generation of Default.                             |               |
| **skipDescription**           | bool | Skip auto-generation of Description.                         |               |
| **skipRequired**              | bool | Skip auto-generation of Required.                            | True          |
| **skipTitle**                 | bool | Skip auto-generation of Title.                               |               |
| **uncommentYAMLBlocks**       | bool | Consider yaml which is commented out.                        |               |

<!-- Auto generated by kcl-doc tool, please do not edit. -->
