package os

import (
	"fmt"

	"kcl-lang.io/kcl-go/pkg/plugin"
)

func init() {
	plugin.RegisterPlugin(plugin.Plugin{
		Name: "os",
		MethodMap: map[string]plugin.MethodSpec{
			"exec": {
				Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
					name := args.StrArg(0)
					strArgs := []string{}
					for _, v := range args.ListArg(1) {
						strArgs = append(strArgs, fmt.Sprint(v))
					}

					exec, err := Exec(name, strArgs...)

					return &plugin.MethodResult{V: map[string]string{
						"stdout": exec.Stdout,
						"stderr": exec.Stderr,
					}}, err
				},
			},
		},
	})
}
