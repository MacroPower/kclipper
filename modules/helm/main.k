"""
This module provides an interface for the macropower/kclx Helm plugin.
"""
import regex
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
    helmVersion: str, default is "v3", optional.
        HelmVersion is the version of Helm to use. One of: [v2, v3].
    skipCRDs: bool, default is False, optional.
        Set to `True` to skip the custom resource definition installation step
        (Helm's `--skip-crds`).
    passCredentials: bool, default is False, optional.
        Set to `True` to pass credentials to all domains (Helm's `--pass-credentials`).
    values: any, default is {}, optional.
        Specifies Helm values to be passed to helm template.
    """
    chart: str
    repoURL: str
    targetRevision: str
    releaseName?: str
    namespace?: str
    helmVersion?: str = "v3"
    skipCRDs?: bool = False
    passCredentials?: bool = False

    check:
        not regex.match(repoURL, r"^oci://"), "Invalid repoURL: ${repoURL}. OCI registries must not include a scheme (e.g. `oci://`)"
        any x in ["v2", "v3"] {
            x == helmVersion
        }, "Invalid helmVersion: ${helmVersion}. Must be one of: [v2, v3]"

schema Chart(ChartBase):
    """Helm chart resource.

    Attributes
    ----------
    values: any, default is {}, optional.
        Specifies Helm values to be passed to helm template.
    """
    values?: any = {}
    preRenderer: (Chart) -> Chart = lambda _chart: Chart -> Chart {
        _chart
    }
    postRenderer: ([{str:}]) -> [{str:}] = lambda _resources: [{str:}] -> [{str:}] {
        _resources
    }

schema ChartConfig(ChartBase):
    r"""
    Helm Chart Configuration

    Attributes
    ----------
    schemaGenerator : "AUTO" | "VALUE-INFERENCE" | "URL" | "PATH" | "LOCAL-PATH" | "NONE", default is "AUTO"
        The generator to use for the Values schema.
    schemaPath : str, optional.
        The path to the JSON Schema to use when schemaGenerator is "URL", "PATH", or "LOCAL-PATH".
    """
    schemaGenerator: "AUTO" | "VALUE-INFERENCE" | "URL" | "PATH" | "LOCAL-PATH" | "NONE" = "AUTO"
    schemaPath?: str

type Charts = {str:ChartConfig}

template = lambda chart: Chart -> [{str:}] {
    """Render Helm chart templates using `macropower/kclx`'s `kcl_plugin.helm.template`.

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
    _chart = chart.preRenderer(chart)

    _resources = helm_plugin.template(chart=_chart.chart, repo_url=_chart.repoURL, target_revision=_chart.targetRevision, release_name=_chart.releaseName, namespace=_chart.namespace, project=_chart.project, helm_version=_chart.helmVersion, skip_crds=_chart.skipCRDs, pass_credentials=_chart.passCredentials, values=_chart.values)

    chart.postRenderer(_resources)
}
