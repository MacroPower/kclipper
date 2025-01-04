import kcl_plugin.helm
import manifests


schema PodInfoValues:
    replicaCount = 1
    logLevel = "info"
    host = None
    backend = None
    backends = [
    ]
    image = {
        repository = "ghcr.io/stefanprodan/podinfo"
        tag = "6.7.1"
        pullPolicy = "IfNotPresent"
    }
    ui = {
        color = "#34577c"
        message = ""
        logo = ""
    }
    faults = {
        delay = False
        error = False
        unhealthy = False
        unready = False
        testFail = False
        testTimeout = False
    }
    service = {
        enabled = True
        annotations = {
        }
        $type = "ClusterIP"
        metricsPort = 9797
        httpPort = 9898
        externalPort = 9898
        grpcPort = 9999
        grpcService = "podinfo"
        nodePort = 31198
        hostPort = None
    }
    h2c = {
        enabled = False
    }
    config = {
        path = ""
        name = ""
    }
    extraArgs = [
    ]
    tls = {
        enabled = False
        secretName = None
        certPath = "/data/cert"
        port = 9899
        hostPort = None
    }
    certificate = {
        create = False
        issuerRef = {
            kind = "ClusterIssuer"
            name = "self-signed"
        }
        dnsNames = [
            "podinfo"
        ]
    }
    hpa = {
        enabled = False
        maxReplicas = 10
        cpu = None
        memory = None
        requests = None
    }
    cache = ""
    redis = {
        enabled = False
        repository = "redis"
        tag = "7.0.7"
    }
    serviceAccount = {
        enabled = False
        name = None
        imagePullSecrets = [
        ]
    }
    securityContext = {
    }
    ingress = {
        enabled = True
        className = ""
        additionalLabels = {
        }
        annotations = {
        }
        hosts = [
            {
                host = "podinfo.local"
                paths = [
                    {
                        path = "/"
                        pathType = "ImplementationSpecific"
                    }
                ]
            }
        ]
        tls = [
        ]
    }
    linkerd = {
        profile = {
            enabled = False
        }
    }
    serviceMonitor = {
        enabled = False
        interval = "15s"
        additionalLabels = {
        }
    }
    resources = {
        limits = None
        requests = {
            cpu = "1m"
            memory = "16Mi"
        }
    }
    extraEnvs = [
    ]
    nodeSelector = {
    }
    tolerations = [
    ]
    affinity = {
    }
    podAnnotations = {
    }
    topologySpreadConstraints = [
    ]
    podDisruptionBudget = {
    }
    probes = {
        readiness = {
            initialDelaySeconds = 1
            timeoutSeconds = 5
            failureThreshold = 3
            successThreshold = 1
            periodSeconds = 10
        }
        liveness = {
            initialDelaySeconds = 1
            timeoutSeconds = 5
            failureThreshold = 3
            successThreshold = 1
            periodSeconds = 10
        }
        startup = {
            enable = False
            initialDelaySeconds = 10
            timeoutSeconds = 5
            failureThreshold = 20
            successThreshold = 1
            periodSeconds = 10
        }
    }

_chart = [helm.template(
    chart="podinfo",
    releaseName="podinfo-${x}",
    target_revision="6.7.1",
    repo_url="https://stefanprodan.github.io/podinfo",
    values=PodInfoValues{}
) for x in range(0, 10)]

manifests.yaml_stream([*_chart])
