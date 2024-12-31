import kcl_plugin.helm

_chart = helm.template(
  chart="wakatime-exporter",
  repo_url="https://jacobcolvin.com/helm-charts",
  target_revision="0.1.0",
  values={service.main.enabled = False},
)

{"result": _chart}
