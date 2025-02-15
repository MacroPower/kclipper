package kclutil

import "kcl-lang.io/kcl-go/pkg/plugin"

type SafeMethodArgs struct {
	Args *plugin.MethodArgs
}

func (sma *SafeMethodArgs) Exists(name string) bool {
	_, ok := sma.Args.KwArgs[name]

	return ok
}

func (sma *SafeMethodArgs) StrKwArg(name, defaultValue string) string {
	if sma.Exists(name) {
		return sma.Args.StrKwArg(name)
	}

	return defaultValue
}

func (sma *SafeMethodArgs) BoolKwArg(name string, defaultValue bool) bool {
	if sma.Exists(name) {
		return sma.Args.BoolKwArg(name)
	}

	return defaultValue
}

func (sma *SafeMethodArgs) MapKwArg(name string, defaultValue map[string]any) map[string]any {
	if sma.Exists(name) {
		return sma.Args.MapKwArg(name)
	}

	return defaultValue
}

func (sma *SafeMethodArgs) ListKwArg(name string, defaultValue []any) []any {
	if sma.Exists(name) {
		return sma.Args.ListKwArg(name)
	}

	return defaultValue
}
