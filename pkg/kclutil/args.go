package kclutil

import (
	"fmt"

	"kcl-lang.io/kcl-go/pkg/plugin"
)

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

func (sma *SafeMethodArgs) StrArg(idx int) (string, error) {
	if len(sma.Args.Args) <= idx {
		return "", fmt.Errorf("expected at least %d argument(s), got %d", idx+1, len(sma.Args.Args))
	}

	arg := sma.Args.Arg(idx)
	result, ok := arg.(string)
	if !ok {
		return "", fmt.Errorf("expected string argument, got %T", arg)
	}

	return result, nil
}

func (sma *SafeMethodArgs) ListStrArg(idx int) ([]string, error) {
	if len(sma.Args.Args) <= idx {
		return nil, fmt.Errorf("expected at least %d argument(s), got %d", idx+1, len(sma.Args.Args))
	}

	arg := sma.Args.Arg(idx)
	result, ok := arg.([]any)
	if !ok {
		return nil, fmt.Errorf("expected []string argument, got %T", arg)
	}

	strResult := make([]string, len(result))
	for i, v := range result {
		strResult[i], ok = v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string at index %d, got %T", i, v)
		}
	}

	return strResult, nil
}
