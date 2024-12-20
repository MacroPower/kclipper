"""
This module provides an interface for the kclx Helm plugin.
"""
import kcl_plugin.helm as helm_plugin

schema Chart:
    """Helm chart resource.

    Attributes
    ----------
    chart: str
        The name of the chart to install.
    repoURL: str
        The URL of the chart repository.
    targetRevision: str
        Version of the chart to install.
    namespace: str, optional.
        Namespace to install the chart into.
    releaseName: str, optional.
        Name of the release.
    skipCRDs: bool, default is False, optional.
        Set to `True` to skip installing CRDs.
    values: any, default is {}, optional.
        Chart values.

    Examples
    --------
    helm.template(helm.Chart {
        chart = "my-chart"
        targetRevision = "1.0.0"
        repoURL = "https://jacobcolvin.com/helm-charts"
        values = {
            foo = "bar"
            bar = "foo"
        }
    })
    """
    chart: str
    targetRevision: str
    repoURL: str
    releaseName?: str
    namespace?: str
    project?: str
    skipCRDs?: bool = False
    values?: any = {}

template = lambda c: Chart -> {str:} {
    helm_plugin.template(
        chart=c.chart,
        target_revision=c.targetRevision,
        repo_url=c.repoURL,
        release_name=c.releaseName,
        namespace=c.namespace,
        project=c.project,
        skip_crds=c.skipCRDs,
        values=c.values,
    )
}
