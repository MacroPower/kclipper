"""
This module provides an interface for the kclx Helm plugin.
"""
import regex
import kcl_plugin.helm as helm_plugin

schema Chart:
    """Helm chart resource.

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
    project: str, optional.
        Project is a reference to the project this chart's releases belong to,
        e.g. the Argo AppProject. This is used to segregate Helm chart caches.
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
    project?: str
    helmVersion?: str = "v3"
    skipCRDs?: bool = False
    passCredentials?: bool = False
    values?: any = {}

    check:
        not regex.match(repoURL, r"^oci://"), \
            "Invalid repoURL: ${repoURL}. OCI registries must not include a scheme (e.g. `oci://`)"
        any x in ["v2", "v3"] {
            x == helmVersion
        }, "Invalid helmVersion: ${helmVersion}. Must be one of: [v2, v3]"

template = lambda chart: Chart -> [{str:}] {
    """Render Helm chart templates using `kclx`'s `kcl_plugin.helm.template`.

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
    helm_plugin.template(
        chart=chart.chart,
        repo_url=chart.repoURL,
        target_revision=chart.targetRevision,
        release_name=chart.releaseName,
        namespace=chart.namespace,
        project=chart.project,
        helm_version=chart.helmVersion,
        skip_crds=chart.skipCRDs,
        pass_credentials=chart.passCredentials,
        values=chart.values,
    )
}
