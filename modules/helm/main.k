"""
This module provides an interface for the kclipper Helm plugin.
"""
import file
import yaml
import k8s.apimachinery.pkg.apis.meta.v1
import json_merge_patch
import kcl_plugin.helm as helm_plugin

type Charts = {str:ChartConfig}

type ChartRepos = {str:ChartRepo}

schema Resource:
    r"""
    Kubernetes resource.

    Attributes
    ----------
    apiVersion : str, required
        Identifies the version of the object's schema.
        More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
    kind : str, required
        Identifies the object's schema.
        More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
    metadata : v1.ObjectMeta, required
        Describes the object's metadata.
        More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
    """

    apiVersion: str
    kind: str
    metadata: v1.ObjectMeta
    [...str]: any


template = lambda chart: Chart -> [Resource] {
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

    if _chart.valueFiles and len(_chart.valueFiles) > 0:
        _values = {
            k: v
            for filename in _chart.valueFiles
            for k, v in json_merge_patch.merge(_values, yaml.decode(file.read(filename)))
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
        skip_hooks=_chart.skipHooks,
        skip_schema_validation=_skipSchemaValidation,
        pass_credentials=_chart.passCredentials,
        repositories=_chart.repositories,
        values=_values,
    )

    if chart.postRenderer:
        _resources = [chart.postRenderer(_resource) for _resource in _resources]

    _resources
}
