package helm_test

import (
	"encoding/json"
	"regexp"
	"testing"

	argocli "github.com/argoproj/argo-cd/v2/util/cli"
	"kcl-lang.io/kcl-go/pkg/spec/gpyrpc"
	"kcl-lang.io/lib/go/native"

	_ "github.com/MacroPower/kclx/pkg/helm"
)

func init() {
	argocli.SetLogLevel("warn")
}

func TestPluginHelmTemplate(t *testing.T) {
	t.Parallel()

	code := `
import kcl_plugin.helm

_chart = helm.template(
  chart="wakatime-exporter",
  repo_url="https://jacobcolvin.com/helm-charts",
  target_revision="0.1.0",
  values={service.main.enabled = False},
)

{result = _chart}
`

	client := native.NewNativeServiceClient()
	result, err := client.ExecProgram(&gpyrpc.ExecProgram_Args{
		KFilenameList: []string{"main.k"},
		KCodeList:     []string{code},
		Args:          []*gpyrpc.Argument{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.GetErrMessage() != "" {
		t.Fatal(result.GetErrMessage())
	}

	wantJSON := `{"result":
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
]}
`

	re := regexp.MustCompile(`\s+`)
	if re.ReplaceAllString(result.GetJsonResult(), "") != re.ReplaceAllString(wantJSON, "") {
		t.Fatal(result.GetJsonResult())
	}
}

func TestExecProgramWithPlugin(t *testing.T) {
	t.Parallel()

	code := `
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

	client := native.NewNativeServiceClient()
	result, err := client.ExecProgram(&gpyrpc.ExecProgram_Args{
		KFilenameList: []string{"main.k"},
		KCodeList:     []string{code},
		Args:          []*gpyrpc.Argument{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.GetErrMessage() != "" {
		t.Fatalf("error message must be empty, got: %s", result.GetErrMessage())
	}

	resultMap := map[string]any{}
	if err := json.Unmarshal([]byte(result.GetJsonResult()), &resultMap); err != nil {
		t.Fatal(err)
	}
	resultChart, ok := resultMap["resources"].([]interface{})
	if !ok {
		t.Fatalf("unexpected type in object: %v", resultMap)
	}
	obj0, ok := resultChart[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected type in object: %v", resultChart)
	}
	obj0md, err := json.Marshal(obj0["metadata"])
	if err != nil {
		t.Fatal(err)
	}
	if string(obj0md) != `{"annotations":{"added":"by kcl"},"labels":{},"name":"wakatime-exporter"}` {
		t.Fatalf("result is not correct, %s", string(obj0md))
	}
}

func BenchmarkPluginHelmTemplate(b *testing.B) {
	code := `
import kcl_plugin.helm

_chart = helm.template(
  chart="wakatime-exporter",
  repo_url="https://jacobcolvin.com/helm-charts",
  target_revision="0.1.0",
  values={service.main.enabled = False},
)

{result = _chart}
`

	client := native.NewNativeServiceClient()
	_, err := client.ExecProgram(&gpyrpc.ExecProgram_Args{
		KFilenameList: []string{"main.k"},
		KCodeList:     []string{code},
		Args:          []*gpyrpc.Argument{},
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client := native.NewNativeServiceClient()
			result, err := client.ExecProgram(&gpyrpc.ExecProgram_Args{
				KFilenameList: []string{"main.k"},
				KCodeList:     []string{code},
				Args:          []*gpyrpc.Argument{},
			})
			if err != nil {
				b.Fatal(err)
			}
			if result.GetErrMessage() != "" {
				b.Fatal(result.GetErrMessage())
			}
		}
	})
}
