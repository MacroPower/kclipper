import helm

repos: helm.ChartRepos = {
    chartmuseum: {
        name = "chartmuseum"
        passwordEnv = "BASIC_AUTH_PASS"
        url = "http://localhost:8080"
        usernameEnv = "BASIC_AUTH_USER"
    }
    prometheus_community: {
        name = "prometheus-community"
        url = "https://prometheus-community.github.io/helm-charts"
    }
}
