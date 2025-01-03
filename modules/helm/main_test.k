test_Chart = lambda {
    https = Chart {
        chart = "test-https"
        repoURL = "https://example.com"
        targetRevision = "0.1.0"
    }

    oci = Chart {
        chart = "test-oci"
        repoURL = "example.com"
        targetRevision = "0.1.0"
    }

    patchRenderer = Chart {
        chart = "test-oci"
        repoURL = "example.com"
        targetRevision = "0.1.0"
        preRenderer = lambda c: Chart {
          c
        }
        postRenderer = lambda r: {str:} {
          r
        }
    }
}
