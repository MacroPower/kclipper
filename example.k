import kcl_plugin.os
import kcl_plugin.helm

{
  "os.exec": os.exec("echo", ["Hello, World!"])
  "os.exec(env)": os.exec("bash", ["-c", 'echo "Hello, $LOCATION!"'], env={"LOCATION": "World"})
  "helm.template": helm.template(
    chart="wakatime-exporter",
    target_revision="0.1.0",
    repo_url="https://jacobcolvin.com/helm-charts",
    values={
      service.main.enabled = False
    },
  )
}
