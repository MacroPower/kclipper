import kcl_plugin.helm

_chart = helm.template(
  chart="wakatime-exporter",
  repo_url="@jacobcolvin",
  target_revision="0.1.0",
  values={service.main.enabled = False},
  repositories=[{
    name="jacobcolvin"
    url="https://jacobcolvin.com/helm-charts"
  }],
)

{"result": _chart}
