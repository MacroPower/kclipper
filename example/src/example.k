import helm
import charts.podinfo
import charts.kube_prometheus_stack.crds as prometheus_crds
import charts.simple_chart
import regex

{
    "helm.template": helm.template(podinfo.Chart {
        values = {
            service.enabled = False
        }
    })
    "helm.template(file)": helm.template(podinfo.Chart {
        valueFiles = ["values.yaml"]
        postRenderer = lambda resource: helm.Resource -> helm.Resource {
            if regex.match(resource.metadata.name, "^podinfo-service-test-.*$"):
                resource.metadata.annotations |= {"example.com/added" = "by kcl patch"}
            resource
        }
    })
    "helm.template(local)": helm.template(simple_chart.Chart {})
    "helm.crd": prometheus_crds.Alertmanager{
      spec: prometheus_crds.MonitoringCoreosComV1AlertmanagerSpec{
        replicas: 2
      }
    }
}
