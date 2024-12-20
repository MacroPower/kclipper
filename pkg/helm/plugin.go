package helm

import (
	"net/url"

	"github.com/pkg/errors"
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
					safeArgs := pluginutil.SafeMethodArgs{Args: args}

					chartName := args.StrKwArg("chart")
					targetRevision := args.StrKwArg("target_revision")
					repoURLStr := args.StrKwArg("repo_url")
					enableOCI := safeArgs.BoolKwArg("enable_oci", false)

					repoURL, err := url.Parse(repoURLStr)
					if err != nil {
						return nil, errors.Wrapf(err, "failed to parse repo_url: %s", repoURLStr)
					}
					if repoURL.Scheme == "" {
						enableOCI = true
					}

					objs, err := DefaultHelm.Template(&TemplateOpts{
						ChartName:       chartName,
						TargetRevision:  targetRevision,
						RepoURL:         repoURL.String(),
						ReleaseName:     safeArgs.StrKwArg("release_name", chartName),
						Namespace:       safeArgs.StrKwArg("namespace", ""),
						Project:         safeArgs.StrKwArg("project", ""),
						HelmVersion:     safeArgs.StrKwArg("helm_version", "v3"),
						EnableOCI:       enableOCI,
						SkipCRDs:        safeArgs.BoolKwArg("skip_crds", false),
						PassCredentials: safeArgs.BoolKwArg("pass_credentials", false),
						ValuesObject:    safeArgs.MapKwArg("values", map[string]any{}),
					})
					if err != nil {
						return nil, err
					}

					return &plugin.MethodResult{V: objs}, nil
				},
			},
		},
	})
}
