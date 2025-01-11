import kcl_plugin.os
import kcl_plugin.http
import helm
import charts.podinfo
import charts.kube_prometheus_stack.crds as prometheus_crds
import regex

{
    "os.exec": os.exec("echo", ["Hello, World!"])
    "os.exec(env)": os.exec("bash", ["-c", 'echo "Hello, $LOCATION!"'], env={"LOCATION": "World"})
    "http.get": http.get("https://example.com", timeout="10s")
    "helm.template": helm.template(podinfo.Chart {
        values = {
            service.enabled = False
        }
    })
    "helm.template(file)": helm.template(podinfo.Chart {
        valueFiles = ["values.yaml"]
        postRenderer = lambda resource: {str:} -> {str:} {
            if regex.match(resource.metadata.name, "^podinfo-service-test-.*$"):
                resource.metadata.annotations |= {"example.com/added" = "by kcl patch"}
            resource
        }
    })
    "helm.crd": prometheus_crds.Alertmanager{
      spec: prometheus_crds.MonitoringCoreosComV1AlertmanagerSpec{
        replicas: 2
      }
    }
}
