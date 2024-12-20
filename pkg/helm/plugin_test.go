package helm_test

import (
	"encoding/json"
	"regexp"
	"testing"

	"kcl-lang.io/kcl-go/pkg/plugin"
	"kcl-lang.io/kcl-go/pkg/spec/gpyrpc"
	"kcl-lang.io/lib/go/native"

	_ "github.com/MacroPower/kclx/pkg/helm"
)

func TestPluginHelmTemplate(t *testing.T) {
	t.Parallel()

	resultJSON := plugin.Invoke("kcl_plugin.helm.template", []interface{}{}, map[string]interface{}{
		"chart":           "wakatime-exporter",
		"repo_url":        "https://jacobcolvin.com/helm-charts",
		"target_revision": "0.1.0",
		"values": map[string]interface{}{
			"service": map[string]interface{}{
				"main": map[string]interface{}{
					"enabled": false,
				},
			},
		},
	})

	re := regexp.MustCompile(`\s+`)
	wantJSON := `
[
  {
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
]
`

	if resultJSON != re.ReplaceAllString(wantJSON, "") {
		t.Fatal(resultJSON)
	}
}

const code = `
import kcl_plugin.helm

_chart = helm.template(
  chart="wakatime-exporter",
  repo_url="https://jacobcolvin.com/helm-charts",
  target_revision="0.1.0",
)

patch = lambda resource: {str:} -> {str:} {
  if resource.kind == "Service":
    resource.metadata.annotations = {
      added = "by kcl"
    }
    resource.metadata.labels = {}

  resource
}

{"resources": [patch(r) for r in _chart]}
`

func TestExecProgramWithPlugin(t *testing.T) {
	client := native.NewNativeServiceClient()
	result, err := client.ExecProgram(&gpyrpc.ExecProgram_Args{
		KFilenameList: []string{"main.k"},
		KCodeList:     []string{code},
		Args:          []*gpyrpc.Argument{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ErrMessage != "" {
		t.Fatalf("error message must be empty, got: %s", result.ErrMessage)
	}
	resultMap := map[string]any{}
	json.Unmarshal([]byte(result.GetJsonResult()), &resultMap)
	resultChart := resultMap["resources"].([]interface{})
	obj0 := resultChart[0].(map[string]interface{})
	obj0md, err := json.Marshal(obj0["metadata"])
	if err != nil {
		t.Fatal(err)
	}
	if string(obj0md) != `{"annotations":{"added":"by kcl"},"labels":{},"name":"wakatime-exporter"}` {
		t.Fatalf("result is not correct, %s", string(obj0md))
	}
}
