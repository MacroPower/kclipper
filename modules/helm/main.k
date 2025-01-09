"""
This module provides an interface for the kclipper Helm plugin.
"""
import file
import yaml
import kcl_plugin.helm as helm_plugin

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
