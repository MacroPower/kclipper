import kcl_plugin.helm

_chart = helm.template(
  chart="simple-chart",
  repo_url="@local",
  repositories=[{
    name="local"
    url="./charts"
  }],
)

{"result": _chart}
