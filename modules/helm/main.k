"""
This module provides an interface for the kclipper Helm plugin.
"""
import regex
import file
import yaml
import kcl_plugin.helm as helm_plugin

schema ChartBase:
    r"""Helm chart resource.

    Attributes
    ----------
    chart: str
        The Helm chart name.
    repoURL: str
        The URL of the Helm chart repository.
    targetRevision: str
        TargetRevision defines the semver tag for the chart's version.
    releaseName: str, optional.
        The Helm release name to use. If omitted it will use the chart name.
    namespace: str, optional.
        Namespace is an optional namespace to template with.
    skipCRDs: bool, default is False, optional.
        Set to `True` to skip the custom resource definition installation step
        (Helm's `--skip-crds`).
    passCredentials: bool, default is False, optional.
        Set to `True` to pass credentials to all domains (Helm's `--pass-credentials`).
    schemaValidator : "KCL" | "HELM", default is "KCL", optional.
        The schema validator to use.
    """
    chart: str
    repoURL: str
    targetRevision: str
    releaseName?: str
    namespace?: str
    skipCRDs?: bool = False
    passCredentials?: bool = False
    schemaValidator?: "KCL" | "HELM"

    check:
        not regex.match(repoURL, r"^oci://"), \
          "Invalid repoURL: ${repoURL}. OCI registries must not include a scheme (e.g. `oci://`)"

schema Chart(ChartBase):
    """Helm chart resource.

    Attributes
    ----------
    values: any, default is {}, optional.
        Specifies Helm values to be passed to Helm template. These take precedence over valueFiles.
    valueFiles: [str], default is [], optional.
        Specifies Helm value files to be passed to Helm template.
    preRenderer: (Chart) -> Chart, optional.
        Lambda function to modify Chart before rendering the Helm template.
    postRenderer: ({str:}) -> {str:}, optional.
        Lambda function to modify the Helm template output. Evaluated for each resource in the Helm template output.
    """
    values?: any = {}
    valueFiles?: [str] = []
    preRenderer?: (Chart) -> Chart
    postRenderer?: ({str:}) -> {str:}

schema ChartConfig(ChartBase):
    r"""
    Helm Chart Configuration

    Attributes
    ----------
    schemaGenerator : "AUTO" | "VALUE-INFERENCE" | "URL" | "CHART-PATH" | "LOCAL-PATH" | "NONE", optional, default is "AUTO"
        The generator to use for the Values schema.
    schemaPath : str, optional.
        The path to the JSON Schema to use when schemaGenerator is "URL", "CHART-PATH", or "LOCAL-PATH".
    """
    schemaGenerator?: "AUTO" | "VALUE-INFERENCE" | "URL" | "CHART-PATH" | "LOCAL-PATH" | "NONE"
    schemaPath?: str

type Charts = {str:ChartConfig}

template = lambda chart: Chart -> [{str:}] {
    """Render Helm chart templates using kclipper's `kcl_plugin.helm.template`.

    Examples
    --------
    ```kcl
    helm.template(helm.Chart {
        chart = "my-chart"
        repoURL = "https://jacobcolvin.com/helm-charts"
        targetRevision = "1.0.0"
        values = {
            foo = "bar"
            bar = "foo"
        }
    })
    ```
    """
    _chart = chart
    _values: {str:} = {}

    if chart.preRenderer:
      _chart = chart.preRenderer(_chart)

    if _chart.valueFiles and len(_chart.valueFiles) > 0:
        _values = {
            k: v
            for filename in _chart.valueFiles
            for k, v in merge(_values, yaml.decode(file.read(filename)))
        }

    _values |= _chart.values

    _skipSchemaValidation = True
    if _chart.schemaValidator:
      _skipSchemaValidation = _chart.schemaValidator == "HELM"

    _resources = helm_plugin.template(
        chart=_chart.chart,
        repo_url=_chart.repoURL,
        target_revision=_chart.targetRevision,
        release_name=_chart.releaseName,
        namespace=_chart.namespace,
        skip_crds=_chart.skipCRDs,
        skip_schema_validation=_skipSchemaValidation,
        pass_credentials=_chart.passCredentials,
        values=_values,
    )

    if chart.postRenderer:
        _resources = [chart.postRenderer(_resource) for _resource in _resources]

    _resources
}
