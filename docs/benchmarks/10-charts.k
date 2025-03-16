import kcl_plugin.helm
import manifests

_chart = [helm.template(
    chart="podinfo",
    releaseName="podinfo-${x}",
    target_revision="6.7.1",
    repo_url="https://stefanprodan.github.io/podinfo",
    values={
        ingress.enabled = True
    }
) for x in range(0, 10)]

manifests.yaml_stream([*_chart])
