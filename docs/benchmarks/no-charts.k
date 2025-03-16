import manifests

_notAChart = [{
    apiVersion = "apps/v1"
    kind = "Deployment"
    metadata = {
        name = "example-${x}"
        labels = {
            "app.kubernetes.io/name": "kcl-example"
            "app.kubernetes.io/instance": "kcl-example"
            "app.kubernetes.io/component": "main_${x}"
            "app.kubernetes.io/managed-by": "KCL"
        }
    }
    spec = {
        replicas = 1
        revisionHistoryLimit = 3
        strategy.type = "Recreate"
        selector.matchLabels = metadata.labels
        template.metadata.labels = metadata.labels
        template.spec = {
            automountServiceAccountToken = True
            containers = [
                {
                    name = metadata.name
                    image = "${metadata.name}:1.14.2"
                    ports = [{containerPort = 80}]
                }
            ]
            dnsPolicy = "ClusterFirst"
            enableServiceLinks = False
            hostIPC = False
            hostNetwork = False
            hostPID = False
            serviceAccountName = "default"
        }
    }
} for x in range(0, 50)]

manifests.yaml_stream(_notAChart, {})
