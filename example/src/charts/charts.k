import helm

charts: helm.Charts = {
    podinfo: {
        chart = "podinfo"
        repoURL = "https://stefanprodan.github.io/podinfo"
        targetRevision = "6.7.1"
        schemaGenerator = "AUTO"
    }
    app_template: {
        chart = "app-template"
        repoURL = "https://bjw-s.github.io/helm-charts/"
        targetRevision = "3.6.0"
        schemaGenerator = "CHART-PATH"
        schemaPath = "charts/common/values.schema.json"
    }
}
