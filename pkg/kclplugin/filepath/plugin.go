package filepathplugin

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"kcl-lang.io/kcl-go/pkg/plugin"

	"github.com/macropower/kclipper/pkg/kclplugin/plugins"
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
				logger := slog.With(
					slog.String("plugin", "filepath"),
					slog.String("method", "base"),
				)
				logger.Debug("invoking kcl plugin")

				safeArgs := plugins.SafeMethodArgs{Args: args}

				filepathStr, err := safeArgs.StrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				result := filepath.Base(filepathStr)

				logger.Debug("returning results")

				return &plugin.MethodResult{V: result}, nil
			},
		},
		"clean": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"str"},
				ResultType: "str",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				logger := slog.With(
					slog.String("plugin", "filepath"),
					slog.String("method", "clean"),
				)
				logger.Debug("invoking kcl plugin")

				safeArgs := plugins.SafeMethodArgs{Args: args}

				filepathStr, err := safeArgs.StrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				result := filepath.Clean(filepathStr)

				logger.Debug("returning results")

				return &plugin.MethodResult{V: result}, nil
			},
		},
		"dir": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"str"},
				ResultType: "str",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				logger := slog.With(
					slog.String("plugin", "filepath"),
					slog.String("method", "dir"),
				)
				logger.Debug("invoking kcl plugin")

				safeArgs := plugins.SafeMethodArgs{Args: args}

				filepathStr, err := safeArgs.StrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				result := filepath.Dir(filepathStr)

				logger.Debug("returning results")

				return &plugin.MethodResult{V: result}, nil
			},
		},
		"ext": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"str"},
				ResultType: "str",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				logger := slog.With(
					slog.String("plugin", "filepath"),
					slog.String("method", "ext"),
				)
				logger.Debug("invoking kcl plugin")

				safeArgs := plugins.SafeMethodArgs{Args: args}

				filepathStr, err := safeArgs.StrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				result := filepath.Ext(filepathStr)

				logger.Debug("returning results")

				return &plugin.MethodResult{V: result}, nil
			},
		},
		"join": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"[str]"},
				ResultType: "str",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				logger := slog.With(
					slog.String("plugin", "filepath"),
					slog.String("method", "join"),
				)
				logger.Debug("invoking kcl plugin")

				safeArgs := plugins.SafeMethodArgs{Args: args}

				filepaths, err := safeArgs.ListStrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				result := filepath.Join(filepaths...)

				logger.Debug("returning results")

				return &plugin.MethodResult{V: result}, nil
			},
		},
		"split": {
			Type: &plugin.MethodType{
				ArgsType:   []string{"str"},
				ResultType: "[str]",
			},
			Body: func(args *plugin.MethodArgs) (*plugin.MethodResult, error) {
				logger := slog.With(
					slog.String("plugin", "filepath"),
					slog.String("method", "split"),
				)
				logger.Debug("invoking kcl plugin")

				safeArgs := plugins.SafeMethodArgs{Args: args}
				filepathStr, err := safeArgs.StrArg(0)
				if err != nil {
					return nil, NewInvalidArgumentError(err)
				}

				dir, file := filepath.Split(filepathStr)

				logger.Debug("returning results")

				return &plugin.MethodResult{V: []string{dir, file}}, nil
			},
		},
	},
}
