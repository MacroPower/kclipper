package filepath

import (
	"fmt"
	"log/slog"

	"kcl-lang.io/kcl-go/pkg/plugin"

	pf "path/filepath"

	"github.com/macropower/kclipper/pkg/kclplugin/plugins"
)

// Register registers the filepath [Plugin] with the KCL plugin system.
func Register() {
	plugin.RegisterPlugin(Plugin)
}

// Plugin is the KCL plugin that exposes Go [path/filepath] functions.
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
					return nil, fmt.Errorf("invalid argument: %w", err)
				}

				result := pf.Base(filepathStr)

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
					return nil, fmt.Errorf("invalid argument: %w", err)
				}

				result := pf.Clean(filepathStr)

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
					return nil, fmt.Errorf("invalid argument: %w", err)
				}

				result := pf.Dir(filepathStr)

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
					return nil, fmt.Errorf("invalid argument: %w", err)
				}

				result := pf.Ext(filepathStr)

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
					return nil, fmt.Errorf("invalid argument: %w", err)
				}

				result := pf.Join(filepaths...)

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
					return nil, fmt.Errorf("invalid argument: %w", err)
				}

				dir, file := pf.Split(filepathStr)

				logger.Debug("returning results")

				return &plugin.MethodResult{V: []string{dir, file}}, nil
			},
		},
	},
}
