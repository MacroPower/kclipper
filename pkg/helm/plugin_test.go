package helm_test

import (
	"regexp"
	"testing"

	"kcl-lang.io/kcl-go/pkg/plugin"

	_ "github.com/MacroPower/kclx/pkg/helm"
)

func TestPluginHelmTemplate(t *testing.T) {
	t.Parallel()

	resultJSON := plugin.Invoke("kcl_plugin.helm.template", []interface{}{}, map[string]interface{}{
		"chart":           "wakatime-exporter",
		"repo_url":        "https://jacobcolvin.com/helm-charts",
		"target_revision": "0.1.0",
		"values_object": map[string]interface{}{
			"service": map[string]interface{}{
				"main": map[string]interface{}{
					"enabled": false,
				},
			},
		},
	})

	re := regexp.MustCompile(`\s+`)
	wantJSON := `
{
  "apps_deployment_wakatime_exporter": {
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
      "labels": {
        "app.kubernetes.io/instance": "wakatime-exporter",
        "app.kubernetes.io/managed-by": "Helm",
        "app.kubernetes.io/name": "wakatime-exporter",
        "app.kubernetes.io/version": "0.1.0",
        "helm.sh/chart": "wakatime-exporter-0.1.0"
      },
      "name": "wakatime-exporter"
    },
    "spec": {
      "replicas": 1,
      "revisionHistoryLimit": 3,
      "selector": {
        "matchLabels": {
          "app.kubernetes.io/instance": "wakatime-exporter",
          "app.kubernetes.io/name": "wakatime-exporter"
        }
      },
      "strategy": { "type": "Recreate" },
      "template": {
        "metadata": {
          "labels": {
            "app.kubernetes.io/instance": "wakatime-exporter",
            "app.kubernetes.io/name": "wakatime-exporter"
          }
        },
        "spec": {
          "automountServiceAccountToken": true,
          "containers": [
            {
              "env": [
                {
                  "name": "WAKA_API_KEY",
                  "valueFrom": {
                    "secretKeyRef": {
                      "key": "api-key",
                      "name": "wakatime-credentials"
                    }
                  }
                }
              ],
              "image": "macropower/wakatime_exporter:0.1.0",
              "imagePullPolicy": "IfNotPresent",
              "name": "wakatime-exporter"
            }
          ],
          "dnsPolicy": "ClusterFirst",
          "enableServiceLinks": true,
          "serviceAccountName": "default"
        }
      }
    }
  }
}
`

	if resultJSON != re.ReplaceAllString(wantJSON, "") {
		t.Fatal(resultJSON)
	}
}
