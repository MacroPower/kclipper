package helm

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"kcl-lang.io/kcl-go/pkg/plugin"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/kclmodule/kclhelm"
	"github.com/MacroPower/kclipper/pkg/kclplugin/plugins"
	"github.com/MacroPower/kclipper/pkg/paths"
)

const (
	argChart                string = "chart"
	argRepoURL              string = "repo_url"
	argTargetRevision       string = "target_revision"
	argReleaseName          string = "release_name"
	argNamespace            string = "namespace"
	argSkipCRDs             string = "skip_crds"
	argSkipSchemaValidation string = "skip_schema_validation"
	argSkipHooks            string = "skip_hooks"
	argPassCredentials      string = "pass_credentials"
	argRepositories         string = "repositories"
	argValues               string = "values"
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
					argChart:                "str",
					argTargetRevision:       "str",
					argRepoURL:              "str",
					argReleaseName:          "str",
					argNamespace:            "str",
					argSkipCRDs:             "bool",
					argSkipSchemaValidation: "bool",
					argSkipHooks:            "bool",
					argPassCredentials:      "bool",
					argRepositories:         "[any]",
					argValues:               "{str:any}",
				},
				ResultType: "[{str:any}]",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				logger := slog.With(
					slog.String("plugin", "helm"),
					slog.String("method", "template"),
				)
				logger.Debug("invoking kcl plugin")

				safeArgs := plugins.SafeMethodArgs{Args: args}

				var merr error

				if !safeArgs.Exists(argChart) {
					merr = multierror.Append(merr, fmt.Errorf("missing required argument: %s", argChart))
				}
				if !safeArgs.Exists(argRepoURL) {
					merr = multierror.Append(merr, fmt.Errorf("missing required argument: %s", argRepoURL))
				}

				chartName := args.StrKwArg(argChart)
				logger = logger.With(
					slog.String(argChart, chartName),
				)

				repoURL := args.StrKwArg(argRepoURL)
				targetRevision := safeArgs.StrKwArg(argTargetRevision, "")
				repos := safeArgs.ListKwArg(argRepositories, []any{})
				releaseName := safeArgs.StrKwArg(argReleaseName, chartName)
				skipCRDs := safeArgs.BoolKwArg(argSkipCRDs, false)
				skipSchemaValidation := safeArgs.BoolKwArg(argSkipSchemaValidation, true)
				skipHooks := safeArgs.BoolKwArg(argSkipHooks, false)
				passCredentials := safeArgs.BoolKwArg(argPassCredentials, false)
				values := safeArgs.MapKwArg(argValues, map[string]any{})

				// https://argo-cd.readthedocs.io/en/stable/user-guide/build-environment/
				// https://github.com/argoproj/argo-cd/pull/15186
				project := os.Getenv("ARGOCD_APP_PROJECT_NAME")
				namespace := safeArgs.StrKwArg(argNamespace, os.Getenv("ARGOCD_APP_NAMESPACE"))
				kubeVersion := os.Getenv("KUBE_VERSION")
				kubeAPIVersions := os.Getenv("KUBE_API_VERSIONS")

				timeoutStr, ok := os.LookupEnv("ARGOCD_EXEC_TIMEOUT")
				if !ok {
					timeoutStr = "60s"
				}
				timeout, err := time.ParseDuration(timeoutStr)
				if err != nil {
					merr = multierror.Append(merr, fmt.Errorf("failed to parse timeout: %w", err))
				}

				if merr != nil {
					return nil, merr
				}

				cwd := os.Getenv("ARGOCD_APP_SOURCE_PATH")
				if cwd == "" {
					cwd = "."
				}
				repoRoot, err := paths.FindRepoRoot(cwd)
				if err != nil {
					return nil, fmt.Errorf("failed to find repository root: %w", err)
				}
				pkgPath, err := paths.FindTopPkgRoot(repoRoot, cwd)
				if err != nil {
					return nil, fmt.Errorf("failed to find package root: %w", err)
				}

				logger.Debug("set arguments",
					slog.String(argRepoURL, repoURL),
					slog.String(argTargetRevision, targetRevision),
					slog.String(argNamespace, namespace),
					slog.String(argReleaseName, releaseName),
					slog.Bool(argSkipCRDs, skipCRDs),
					slog.Bool(argSkipSchemaValidation, skipSchemaValidation),
					slog.Bool(argSkipHooks, skipHooks),
					slog.Bool(argPassCredentials, passCredentials),
					slog.String("project", project),
					slog.String("kube_version", kubeVersion),
					slog.String("kube_api_versions", kubeAPIVersions),
					slog.String("cwd", cwd),
					slog.String("pkg_path", pkgPath),
					slog.String("repo_root", repoRoot),
					slog.String("timeout", timeout.String()),
				)

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

				tempPaths := paths.NewStaticTempPaths(filepath.Join(os.TempDir(), "charts"), paths.NewBase64PathEncoder())
				helmClient, err := helm.NewClient(tempPaths, project)
				if err != nil {
					return nil, fmt.Errorf("failed to create helm client: %w", err)
				}

				helmChart, err := helm.NewChart(helmClient, repoMgr, &helm.TemplateOpts{
					ChartName:            chartName,
					TargetRevision:       targetRevision,
					RepoURL:              repoURL,
					ReleaseName:          releaseName,
					Namespace:            namespace,
					SkipCRDs:             skipCRDs,
					SkipSchemaValidation: skipSchemaValidation,
					SkipHooks:            skipHooks,
					PassCredentials:      passCredentials,
					ValuesObject:         values,
					KubeVersion:          kubeVersion,
					APIVersions:          strings.Split(kubeAPIVersions, ","),
					Timeout:              timeout,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to create chart handler for %q: %w", chartName, err)
				}

				logger.Info("execute helm template")
				objs, err := helmChart.Template(context.Background())
				if err != nil {
					return nil, fmt.Errorf("failed to template %q: %w", chartName, err)
				}
				logger.Info("helm template complete")

				logger.Debug("returning results")

				return &plugin.MethodResult{V: objs}, nil
			},
		},
	},
}
