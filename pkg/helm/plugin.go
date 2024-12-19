package helm

import (
	"fmt"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"kcl-lang.io/kcl-go/pkg/plugin"

	pluginutil "github.com/MacroPower/kclx/pkg/util/plugin"
)

func init() {
	plugin.RegisterPlugin(plugin.Plugin{
		Name: "helm",
		MethodMap: map[string]plugin.MethodSpec{
			"template": {
				Type: &plugin.MethodType{
					KwArgsType: map[string]string{
						"chart":            "str",
						"target_revision":  "str",
						"repo_url":         "str",
						"release_name":     "str",
						"namespace":        "str",
						"project":          "str",
						"helm_version":     "str",
						"enable_oci":       "bool",
						"skip_crds":        "bool",
						"pass_credentials": "bool",
						"values":           "{str:any}",
					},
					ResultType: "{str:any}",
				},
				Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
					chartName := args.StrKwArg("chart")
					targetRevision := args.StrKwArg("target_revision")
					repoURL := args.StrKwArg("repo_url")

					safeArgs := pluginutil.SafeMethodArgs{Args: args}

					objs, err := DefaultHelm.Template(&TemplateOpts{
						ChartName:       chartName,
						TargetRevision:  targetRevision,
						RepoURL:         repoURL,
						ReleaseName:     safeArgs.StrKwArg("release_name", chartName),
						Namespace:       safeArgs.StrKwArg("namespace", ""),
						Project:         safeArgs.StrKwArg("project", ""),
						HelmVersion:     safeArgs.StrKwArg("helm_version", "v3"),
						EnableOCI:       safeArgs.BoolKwArg("enable_oci", false),
						SkipCRDs:        safeArgs.BoolKwArg("skip_crds", false),
						PassCredentials: safeArgs.BoolKwArg("pass_credentials", false),
						ValuesObject:    safeArgs.MapKwArg("values", map[string]any{}),
					})
					if err != nil {
						return nil, err
					}

					objMap := make(map[string]*unstructured.Unstructured, len(objs))
					for _, obj := range objs {
						rk := kube.GetResourceKey(obj)
						key := fmt.Sprintf("%s_%s", rk.Kind, rk.Name)
						if rk.Group != "" {
							key = fmt.Sprintf("%s_%s", rk.Group, key)
						}
						key = strings.ToLower(strings.ReplaceAll(key, "-", "_"))
						objMap[key] = obj
					}

					return &plugin.MethodResult{V: objMap}, nil
				},
			},
		},
	})
}
