package kclutil_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kcl-lang.io/kcl-go/pkg/plugin"

	"github.com/MacroPower/kclipper/pkg/kclutil"
)

func TestSafeMethodArgs_Exists(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		args     map[string]any
		argName  string
		expected bool
	}{
		"key exists": {
			args:     map[string]any{"key": "value"},
			argName:  "key",
			expected: true,
		},
		"key does not exist": {
			args:     map[string]any{"other_key": "value"},
			argName:  "key",
			expected: false,
		},
		"empty args": {
			args:     map[string]any{},
			argName:  "key",
			expected: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			methodArgs := &plugin.MethodArgs{
				KwArgs: tc.args,
			}
			safeArgs := kclutil.SafeMethodArgs{Args: methodArgs}

			result := safeArgs.Exists(tc.argName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSafeMethodArgs_StrKwArg(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		args         map[string]any
		argName      string
		defaultValue string
		expected     string
	}{
		"key exists": {
			args:         map[string]any{"key": "value"},
			argName:      "key",
			defaultValue: "default",
			expected:     "value",
		},
		"key does not exist": {
			args:         map[string]any{"other_key": "value"},
			argName:      "key",
			defaultValue: "default",
			expected:     "default",
		},
		"empty args": {
			args:         map[string]any{},
			argName:      "key",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			methodArgs := &plugin.MethodArgs{
				KwArgs: tc.args,
			}
			safeArgs := kclutil.SafeMethodArgs{Args: methodArgs}

			result := safeArgs.StrKwArg(tc.argName, tc.defaultValue)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSafeMethodArgs_BoolKwArg(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		args         map[string]any
		argName      string
		defaultValue bool
		expected     bool
	}{
		"key exists with true": {
			args:         map[string]any{"key": true},
			argName:      "key",
			defaultValue: false,
			expected:     true,
		},
		"key exists with false": {
			args:         map[string]any{"key": false},
			argName:      "key",
			defaultValue: true,
			expected:     false,
		},
		"key does not exist": {
			args:         map[string]any{"other_key": true},
			argName:      "key",
			defaultValue: true,
			expected:     true,
		},
		"empty args": {
			args:         map[string]any{},
			argName:      "key",
			defaultValue: true,
			expected:     true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			methodArgs := &plugin.MethodArgs{
				KwArgs: tc.args,
			}
			safeArgs := kclutil.SafeMethodArgs{Args: methodArgs}

			result := safeArgs.BoolKwArg(tc.argName, tc.defaultValue)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSafeMethodArgs_MapKwArg(t *testing.T) {
	t.Parallel()

	testMap := map[string]any{"nested": "value"}
	defaultMap := map[string]any{"default": "value"}

	tcs := map[string]struct {
		args         map[string]any
		defaultValue map[string]any
		expected     map[string]any
		argName      string
	}{
		"key exists": {
			args:         map[string]any{"key": testMap},
			argName:      "key",
			defaultValue: defaultMap,
			expected:     testMap,
		},
		"key does not exist": {
			args:         map[string]any{"other_key": testMap},
			argName:      "key",
			defaultValue: defaultMap,
			expected:     defaultMap,
		},
		"empty args": {
			args:         map[string]any{},
			argName:      "key",
			defaultValue: defaultMap,
			expected:     defaultMap,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			methodArgs := &plugin.MethodArgs{
				KwArgs: tc.args,
			}
			safeArgs := kclutil.SafeMethodArgs{Args: methodArgs}

			result := safeArgs.MapKwArg(tc.argName, tc.defaultValue)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSafeMethodArgs_ListKwArg(t *testing.T) {
	t.Parallel()

	testList := []any{"value1", "value2"}
	defaultList := []any{"default1", "default2"}

	tcs := map[string]struct {
		args         map[string]any
		argName      string
		defaultValue []any
		expected     []any
	}{
		"key exists": {
			args:         map[string]any{"key": testList},
			argName:      "key",
			defaultValue: defaultList,
			expected:     testList,
		},
		"key does not exist": {
			args:         map[string]any{"other_key": testList},
			argName:      "key",
			defaultValue: defaultList,
			expected:     defaultList,
		},
		"empty args": {
			args:         map[string]any{},
			argName:      "key",
			defaultValue: defaultList,
			expected:     defaultList,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			methodArgs := &plugin.MethodArgs{
				KwArgs: tc.args,
			}
			safeArgs := kclutil.SafeMethodArgs{Args: methodArgs}

			result := safeArgs.ListKwArg(tc.argName, tc.defaultValue)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSafeMethodArgs_StrArg(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		err      error
		expected string
		args     []any
		idx      int
	}{
		"valid string at index": {
			args:     []any{"value1", "value2"},
			idx:      0,
			expected: "value1",
			err:      nil,
		},
		"index out of bounds": {
			args:     []any{"value1"},
			idx:      1,
			expected: "",
			err:      errors.New("expected at least 2 argument(s), got 1"),
		},
		"non-string value": {
			args:     []any{123, "value2"},
			idx:      0,
			expected: "",
			err:      errors.New("expected string argument at index 0, got int"),
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			methodArgs := &plugin.MethodArgs{
				Args: tc.args,
			}
			safeArgs := kclutil.SafeMethodArgs{Args: methodArgs}

			result, err := safeArgs.StrArg(tc.idx)

			if tc.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.err.Error())

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSafeMethodArgs_ListStrArg(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		err      error
		args     []any
		expected []string
		idx      int
	}{
		"valid string list at index": {
			args:     []any{[]any{"value1", "value2"}, "other"},
			idx:      0,
			expected: []string{"value1", "value2"},
			err:      nil,
		},
		"index out of bounds": {
			args:     []any{[]any{"value1"}},
			idx:      1,
			expected: nil,
			err:      errors.New("expected at least 2 argument(s), got 1"),
		},
		"non-list value": {
			args:     []any{"not a list", "value2"},
			idx:      0,
			expected: nil,
			err:      errors.New("expected []string argument at index 0, got string"),
		},
		"non-string in list": {
			args:     []any{[]any{"value1", 123}, "other"},
			idx:      0,
			expected: nil,
			err:      errors.New("expected string at index 1, got int"),
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			methodArgs := &plugin.MethodArgs{
				Args: tc.args,
			}
			safeArgs := kclutil.SafeMethodArgs{Args: methodArgs}

			result, err := safeArgs.ListStrArg(tc.idx)

			if tc.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.err.Error())

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
