test_Chart = lambda {
    https = Chart {
        chart = "test-https"
        repoURL = "https://example.com"
        targetRevision = "0.1.0"
        helmVersion = "v3"
    }

    oci = Chart {
        chart = "test-oci"
        repoURL = "example.com"
        targetRevision = "0.1.0"
    }

    helmV2 = Chart {
        chart = "test-helm-v2"
        repoURL = "https://example.com"
        targetRevision = "0.1.0"
        helmVersion = "v2"
    }
}
