package filepathplugin

import (
	"fmt"
	"path/filepath"

	"kcl-lang.io/kcl-go/pkg/plugin"

	"github.com/MacroPower/kclipper/pkg/kclutil"
)

type InvalidArgumentError struct {
	Err error
}

func NewInvalidArgumentError(err error) *InvalidArgumentError {
	return &InvalidArgumentError{Err: err}
}

func (e *InvalidArgumentError) Error() string {
	return fmt.Sprintf("invalid argument: %v", e.Err)
}

func Register() {
	plugin.RegisterPlugin(Plugin)
}

var Plugin = plugin.Plugin{
	Name: "filepath",
	MethodMap: map[string]plugin.MethodSpec{
		"base": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"str"},
				ResultType: "str",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				safeArgs := kclutil.SafeMethodArgs{Args: args}

				filepathStr, err := safeArgs.StrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				result := filepath.Base(filepathStr)

				return &plugin.MethodResult{V: result}, nil
			},
		},
		"clean": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"str"},
				ResultType: "str",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				safeArgs := kclutil.SafeMethodArgs{Args: args}

				filepathStr, err := safeArgs.StrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				result := filepath.Clean(filepathStr)

				return &plugin.MethodResult{V: result}, nil
			},
		},
		"dir": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"str"},
				ResultType: "str",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				safeArgs := kclutil.SafeMethodArgs{Args: args}

				filepathStr, err := safeArgs.StrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				result := filepath.Dir(filepathStr)

				return &plugin.MethodResult{V: result}, nil
			},
		},
		"ext": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"str"},
				ResultType: "str",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				safeArgs := kclutil.SafeMethodArgs{Args: args}

				filepathStr, err := safeArgs.StrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				result := filepath.Ext(filepathStr)

				return &plugin.MethodResult{V: result}, nil
			},
		},
		"join": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"[str]"},
				ResultType: "str",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				safeArgs := kclutil.SafeMethodArgs{Args: args}

				filepaths, err := safeArgs.ListStrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				result := filepath.Join(filepaths...)

				return &plugin.MethodResult{V: result}, nil
			},
		},
		"split": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"str"},
				ResultType: "[str]",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				safeArgs := kclutil.SafeMethodArgs{Args: args}
				filepathStr, err := safeArgs.StrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				dir, file := filepath.Split(filepathStr)

				return &plugin.MethodResult{V: []string{dir, file}}, nil
			},
		},
	},
}
