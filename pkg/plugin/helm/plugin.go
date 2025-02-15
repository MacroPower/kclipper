package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kcl-lang.io/kcl-go/pkg/plugin"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/kclhelm"
	"github.com/MacroPower/kclipper/pkg/kclutil"
	"github.com/MacroPower/kclipper/pkg/pathutil"
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
					"skip_hooks":             "bool",
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
				repos := safeArgs.ListKwArg("repositories", []any{})

				// https://argo-cd.readthedocs.io/en/stable/user-guide/build-environment/
				// https://github.com/argoproj/argo-cd/pull/15186
				project := os.Getenv("ARGOCD_APP_PROJECT_NAME")
				namespace := safeArgs.StrKwArg("namespace", os.Getenv("ARGOCD_APP_NAMESPACE"))
				kubeVersion := os.Getenv("KUBE_VERSION")
				kubeAPIVersions := os.Getenv("KUBE_API_VERSIONS")

				var err error
				cwd := os.Getenv("ARGOCD_APP_SOURCE_PATH")
				if cwd == "" {
					cwd = "."
				}
				repoRoot, err := kclutil.FindRepoRoot(cwd)
				if err != nil {
					return nil, fmt.Errorf("failed to find repository root: %w", err)
				}
				pkgPath, err := kclutil.FindTopPkgRoot(repoRoot, cwd)
				if err != nil {
					return nil, fmt.Errorf("failed to find package root: %w", err)
				}

				repoMgr := helmrepo.NewManager(helmrepo.WithAllowedPaths(pkgPath, repoRoot))
				for _, repo := range repos {
					var pcr kclhelm.ChartRepo
					repoMap, ok := repo.(map[string]any)
					if !ok {
						return nil, fmt.Errorf("invalid repository: %#v", repo)
					}
					err := pcr.FromMap(repoMap)
					if err != nil {
						return nil, fmt.Errorf("invalid repository: %w", err)
					}
					hr, err := pcr.GetHelmRepo()
					if err != nil {
						return nil, fmt.Errorf("failed to add Helm repository: %w", err)
					}
					if err := repoMgr.Add(hr); err != nil {
						return nil, fmt.Errorf("failed to add Helm repository: %w", err)
					}
				}

				tempPaths := pathutil.NewStaticTempPaths(filepath.Join(os.TempDir(), "charts"), pathutil.NewBase64PathEncoder())
				helmClient, err := helm.NewClient(tempPaths, project, "10M")
				if err != nil {
					return nil, fmt.Errorf("failed to create helm client: %w", err)
				}

				helmChart, err := helm.NewChart(helmClient, repoMgr, &helm.TemplateOpts{
					ChartName:            chartName,
					TargetRevision:       targetRevision,
					RepoURL:              repoURL,
					ReleaseName:          safeArgs.StrKwArg("release_name", chartName),
					Namespace:            namespace,
					SkipCRDs:             safeArgs.BoolKwArg("skip_crds", false),
					SkipSchemaValidation: safeArgs.BoolKwArg("skip_schema_validation", true),
					SkipHooks:            safeArgs.BoolKwArg("skip_hooks", false),
					PassCredentials:      safeArgs.BoolKwArg("pass_credentials", false),
					ValuesObject:         safeArgs.MapKwArg("values", map[string]any{}),
					KubeVersion:          kubeVersion,
					APIVersions:          strings.Split(kubeAPIVersions, ","),
				})
				if err != nil {
					return nil, fmt.Errorf("failed to create chart handler for %q: %w", chartName, err)
				}

				objs, err := helmChart.Template()
				if err != nil {
					return nil, fmt.Errorf("failed to template %q: %w", chartName, err)
				}

				return &plugin.MethodResult{V: objs}, nil
			},
		},
	},
}
