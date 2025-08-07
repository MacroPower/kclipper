import helm

charts: helm.Charts = {
    podinfo: {
        chart = "podinfo"
        repoURL = "https://stefanprodan.github.io/podinfo"
        targetRevision = "6.7.1"
        schemaGenerator = "AUTO"
        schemaValidator = "KCL"
    }
    podinfo_v5: {
        chart = "podinfo"
        repoURL = "https://stefanprodan.github.io/podinfo"
        targetRevision = "5.2.1"
        schemaGenerator = "AUTO"
        schemaValidator = "KCL"
    }
    app_template: {
        chart = "app-template"
        repoURL = "https://bjw-s.github.io/helm-charts/"
        targetRevision = "3.7.1"
        schemaGenerator = "CHART-PATH"
        schemaPath = "charts/common/values.schema.json"
        schemaValidator = "KCL"
    }
    kube_prometheus_stack: {
        chart = "kube-prometheus-stack"
        crdGenerator = "CHART-PATH"
        crdPaths = ["**/crds/crds/*.yaml"]
        repoURL = "https://prometheus-community.github.io/helm-charts"
        schemaGenerator = "AUTO"
        targetRevision = "75.18.1"
    }
    simple_chart: {
        chart = "simple-chart"
        repoURL = "@local"
        repositories = [repos.local]
        schemaGenerator = "VALUE-INFERENCE"
        valueInference: {
            keepFullComment = True
        }
    }
    external_secrets: {
        chart = "external-secrets"
        repoURL = "https://charts.external-secrets.io/"
        schemaGenerator = "AUTO"
        targetRevision = "0.19.0"
        crdGenerator = "TEMPLATE"
        values: {
            installCRDs = True
            crds: {
                createClusterExternalSecret = False
                createClusterGenerator = False
                createClusterSecretStore = False
                createPushSecret = True
            }
        }
    }
}
