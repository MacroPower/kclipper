import kcl_plugin.helm

_chart = helm.template(
  chart="simple-chart",
  repo_url="@local",
  repositories=[{
    name="local"
    url="./testdata/charts"
  }],
)

{"result": _chart}
