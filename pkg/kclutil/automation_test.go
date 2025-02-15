package kclutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/kclutil"
)

func TestNewString(t *testing.T) {
	t.Parallel()

	s := "test"
	mv := kclutil.NewString(s)
	assert.True(t, mv.IsString())
	assert.False(t, mv.IsBool())
	assert.Equal(t, `"test"`, mv.GetValue())
}

func TestNewBool(t *testing.T) {
	t.Parallel()

	b := true
	mv := kclutil.NewBool(b)
	assert.False(t, mv.IsString())
	assert.True(t, mv.IsBool())
	assert.Equal(t, "True", mv.GetValue())
}

func TestAutomationSpecs(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input    kclutil.Automation
		specPath string
		expected []string
		err      bool
	}{
		"valid input": {
			input: kclutil.Automation{
				"key1": kclutil.NewString("value1"),
				"key2": kclutil.NewBool(true),
			},
			specPath: "spec",
			expected: []string{"spec.key1=\"value1\"", "spec.key2=True"},
			err:      false,
		},
		"empty key": {
			input: kclutil.Automation{
				"": kclutil.NewString("value1"),
			},
			specPath: "spec",
			expected: nil,
			err:      true,
		},
		"empty value": {
			input: kclutil.Automation{
				"key1": kclutil.NewString(""),
				"key2": kclutil.NewBool(false),
			},
			specPath: "spec",
			expected: []string{},
			err:      false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			specs, err := tc.input.GetSpecs(tc.specPath)
			if tc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, specs)
			}
		})
	}
}

func TestSpecPathJoin(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input    []string
		expected string
	}{
		"single path": {
			input:    []string{"path"},
			expected: "path",
		},
		"multiple paths": {
			input:    []string{"path", "to", "spec"},
			expected: "path.to.spec",
		},
		"dot separated paths": {
			input:    []string{"path.to.", ".spec"},
			expected: "path.to.spec",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := kclutil.SpecPathJoin(tc.input...)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestKCLAutomation(t *testing.T) {
	t.Parallel()

	testAutomationDir := filepath.Join("testdata", "automation")

	err := os.RemoveAll(testAutomationDir)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(testAutomationDir, ".tmp"), 0o700)
	require.NoError(t, err)

	tcs := map[string]struct {
		input    kclutil.Automation
		specPath string
		inputKCL string
		expected string
	}{
		"valid input": {
			input: kclutil.Automation{
				"key1": kclutil.NewString("value1"),
				"key2": kclutil.NewBool(true),
				"key3": kclutil.NewBool(false),
			},
			inputKCL: ``,
			expected: `{"key1": "value1", "key2": true}`,
		},
		"map keys": {
			input: kclutil.Automation{
				"key1": kclutil.NewString("value1"),
				"key2": kclutil.NewBool(true),
				"key3": kclutil.NewBool(false),
			},
			specPath: "test",
			inputKCL: `test = {}`,
			expected: `{"test": {"key1": "value1", "key2": true}}`,
		},
		"map dicts": {
			input: kclutil.Automation{
				"obj1.key1": kclutil.NewString("value1"),
				"obj1.key2": kclutil.NewBool(true),
				"obj2.key1": kclutil.NewString("value1"),
				"obj2.key2": kclutil.NewBool(true),
				"obj3.key3": kclutil.NewBool(false),
			},
			specPath: "test",
			inputKCL: `test = {}`,
			expected: `{"test": {
				"obj1": {"key1": "value1", "key2": true},
				"obj2": {"key1": "value1", "key2": true}
			}}`,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			inputFile := filepath.Join(testAutomationDir, ".tmp", name+".k")
			err := os.WriteFile(inputFile, []byte(tc.inputKCL), 0o600)
			require.NoErrorf(t, err, "failed to write '%s'", inputFile)

			specs, err := tc.input.GetSpecs(tc.specPath)
			require.NoErrorf(t, err, "failed generating inputs for '%s'", inputFile)

			imports := []string{}
			_, err = kcl.OverrideFile(inputFile, specs, imports)
			require.NoErrorf(t, err, "failed to update '%s'", inputFile)

			got, err := kcl.Run(inputFile)
			require.NoErrorf(t, err, "failed to evaluate '%s'", inputFile)

			assert.JSONEq(t, tc.expected, got.GetRawJsonResult())
		})
	}
}
