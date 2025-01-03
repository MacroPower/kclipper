package helm

import (
	"fmt"
	"os"
	"strings"

	"kcl-lang.io/kcl-go/pkg/plugin"

	"github.com/MacroPower/kclipper/pkg/helm"
	kclutil "github.com/MacroPower/kclipper/pkg/kclutil"
)

func Register() {
	plugin.RegisterPlugin(Plugin)
}

var Plugin = plugin.Plugin{
	Name: "helm",
	MethodMap: map[string]plugin.MethodSpec{
		"template": {
			Type: &plugin.MethodType{
				KwArgsType: map[string]string{
					"chart":                  "str",
					"target_revision":        "str",
					"repo_url":               "str",
					"release_name":           "str",
					"namespace":              "str",
					"skip_crds":              "bool",
					"skip_schema_validation": "bool",
					"pass_credentials":       "bool",
					"values":                 "{str:any}",
				},
				ResultType: "[{str:any}]",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				safeArgs := kclutil.SafeMethodArgs{Args: args}

				chartName := args.StrKwArg("chart")
				targetRevision := args.StrKwArg("target_revision")
				repoURL := args.StrKwArg("repo_url")

				// https://argo-cd.readthedocs.io/en/stable/user-guide/build-environment/
				// https://github.com/argoproj/argo-cd/pull/15186
				project := os.Getenv("ARGOCD_APP_PROJECT_NAME")
				namespace := safeArgs.StrKwArg("namespace", os.Getenv("ARGOCD_APP_NAMESPACE"))
				kubeVersion := os.Getenv("KUBE_VERSION")
				kubeAPIVersions := os.Getenv("KUBE_API_VERSIONS")

				helmClient, err := helm.NewClient(helm.NewTempPaths(os.TempDir(), helm.NewBase64PathEncoder()), project, "10M")
				if err != nil {
					return nil, fmt.Errorf("failed to create helm client: %w", err)
				}

				helmChart := helm.NewChart(helmClient, helm.TemplateOpts{
					ChartName:            chartName,
					TargetRevision:       targetRevision,
					RepoURL:              repoURL,
					ReleaseName:          safeArgs.StrKwArg("release_name", chartName),
					Namespace:            namespace,
					SkipCRDs:             safeArgs.BoolKwArg("skip_crds", false),
					SkipSchemaValidation: safeArgs.BoolKwArg("skip_schema_validation", true),
					PassCredentials:      safeArgs.BoolKwArg("pass_credentials", false),
					ValuesObject:         safeArgs.MapKwArg("values", map[string]any{}),
					KubeVersion:          kubeVersion,
					APIVersions:          strings.Split(kubeAPIVersions, ","),
				})

				objs, err := helmChart.Template()
				if err != nil {
					return nil, fmt.Errorf("failed to template '%s': %w", chartName, err)
				}

				return &plugin.MethodResult{V: objs}, nil
			},
		},
	},
}
