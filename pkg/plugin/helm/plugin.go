package helm

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"kcl-lang.io/kcl-go/pkg/plugin"

	"github.com/MacroPower/kclx/pkg/helm"
	kclutil "github.com/MacroPower/kclx/pkg/kclutil"
)

func init() {
	if strings.ToLower(os.Getenv("KCLX_HELM_PLUGIN_DISABLED")) == "true" {
		return
	}

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
						"helm_version":     "str",
						"enable_oci":       "bool",
						"skip_crds":        "bool",
						"pass_credentials": "bool",
						"values":           "{str:any}",
					},
					ResultType: "[{str:any}]",
				},
				Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
					safeArgs := kclutil.SafeMethodArgs{Args: args}

					chartName := args.StrKwArg("chart")
					targetRevision := args.StrKwArg("target_revision")
					repoURLStr := args.StrKwArg("repo_url")
					enableOCI := safeArgs.BoolKwArg("enable_oci", false)

					// https://argo-cd.readthedocs.io/en/stable/user-guide/build-environment/
					// https://github.com/argoproj/argo-cd/pull/15186
					project := os.Getenv("ARGOCD_APP_PROJECT_NAME")
					namespace := safeArgs.StrKwArg("namespace", os.Getenv("ARGOCD_APP_NAMESPACE"))
					kubeVersion := os.Getenv("KUBE_VERSION")
					kubeAPIVersions := os.Getenv("KUBE_API_VERSIONS")

					repoURL, err := url.Parse(repoURLStr)
					if err != nil {
						return nil, fmt.Errorf("failed to parse repo_url '%s': %w", repoURLStr, err)
					}
					if repoURL.Scheme == "" {
						enableOCI = true
					}

					objs, err := helm.DefaultHelm.Template(&helm.TemplateOpts{
						ChartName:       chartName,
						TargetRevision:  targetRevision,
						RepoURL:         repoURL.String(),
						ReleaseName:     safeArgs.StrKwArg("release_name", chartName),
						Namespace:       namespace,
						Project:         project,
						HelmVersion:     safeArgs.StrKwArg("helm_version", "v3"),
						EnableOCI:       enableOCI,
						SkipCRDs:        safeArgs.BoolKwArg("skip_crds", false),
						PassCredentials: safeArgs.BoolKwArg("pass_credentials", false),
						ValuesObject:    safeArgs.MapKwArg("values", map[string]any{}),
						KubeVersion:     kubeVersion,
						APIVersions:     strings.Split(kubeAPIVersions, ","),
					})
					if err != nil {
						return nil, fmt.Errorf("failed to template '%s': %w", chartName, err)
					}

					return &plugin.MethodResult{V: objs}, nil
				},
			},
		},
	})
}
