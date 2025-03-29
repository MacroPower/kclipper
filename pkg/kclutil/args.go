package kclutil

import (
	"fmt"

	"kcl-lang.io/kcl-go/pkg/plugin"
)

// SafeMethodArgs provides safe access to KCL plugin method arguments.
type SafeMethodArgs struct {
	Args *plugin.MethodArgs
}

// Exists checks if an argument with the given name exists.
func (sma *SafeMethodArgs) Exists(name string) bool {
	_, ok := sma.Args.KwArgs[name]

	return ok
}

// StrKwArg returns the string keyword argument with the given name, or defaultValue if it doesn't exist.
func (sma *SafeMethodArgs) StrKwArg(name, defaultValue string) string {
	if sma.Exists(name) {
		return sma.Args.StrKwArg(name)
	}

	return defaultValue
}

// BoolKwArg returns the boolean keyword argument with the given name, or defaultValue if it doesn't exist.
func (sma *SafeMethodArgs) BoolKwArg(name string, defaultValue bool) bool {
	if sma.Exists(name) {
		return sma.Args.BoolKwArg(name)
	}

	return defaultValue
}

// MapKwArg returns the map keyword argument with the given name, or defaultValue if it doesn't exist.
func (sma *SafeMethodArgs) MapKwArg(name string, defaultValue map[string]any) map[string]any {
	if sma.Exists(name) {
		return sma.Args.MapKwArg(name)
	}

	return defaultValue
}

// ListKwArg returns the list keyword argument with the given name, or defaultValue if it doesn't exist.
func (sma *SafeMethodArgs) ListKwArg(name string, defaultValue []any) []any {
	if sma.Exists(name) {
		return sma.Args.ListKwArg(name)
	}

	return defaultValue
}

// StrArg returns the string argument at the given index.
func (sma *SafeMethodArgs) StrArg(idx int) (string, error) {
	if len(sma.Args.Args) <= idx {
		return "", fmt.Errorf("%w: expected at least %d argument(s), got %d",
			ErrInvalidArguments, idx+1, len(sma.Args.Args))
	}

	arg := sma.Args.Arg(idx)
	result, ok := arg.(string)
	if !ok {
		return "", fmt.Errorf("%w: expected string argument at index %d, got %T",
			ErrInvalidArguments, idx, arg)
	}

	return result, nil
}

// ListStrArg returns the string list argument at the given index.
func (sma *SafeMethodArgs) ListStrArg(idx int) ([]string, error) {
	if len(sma.Args.Args) <= idx {
		return nil, fmt.Errorf("%w: expected at least %d argument(s), got %d",
			ErrInvalidArguments, idx+1, len(sma.Args.Args))
	}

	arg := sma.Args.Arg(idx)
	result, ok := arg.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: expected []string argument at index %d, got %T",
			ErrInvalidArguments, idx, arg)
	}

	strResult := make([]string, len(result))
	for i, v := range result {
		strResult[i], ok = v.(string)
		if !ok {
			return nil, fmt.Errorf("%w: expected string at index %d, got %T",
				ErrInvalidArguments, i, v)
		}
	}

	return strResult, nil
}
