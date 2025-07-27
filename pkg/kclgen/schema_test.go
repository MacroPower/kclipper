package kclgen_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/macropower/kclipper/pkg/kclgen"
)

const (
	schemaWithDefaults = `
schema Test:
    field1: str = "default"
    field2: int = 42
`

	schemaWithNoDefaults = `
schema Test:
    field1: str
    field2: int
`

	schemaWithMultilineDefaults = `
schema Test:
    field1: str = r"""
multi
line
string
"""
    field2: int = 42
`

	expectedWithoutMultilineDefaults = `
schema Test:
    field1: str
    field2: int
`

	schemaWithUnionTypes = `
schema Test:
    field1: str | int = "default"
    field2: str | int | bool = 42
`

	expectedWithoutUnionTypeDefaults = `
schema Test:
    field1: str | int
    field2: str | int | bool
`

	schemaWithMixedContent = `
schema Test:
    # A string field
    field1: str = "default" # with comment

    # A multiline field
    field2: str = r"""
multi
line
string
"""

    # A number field
    field3: int = 42
`

	expectedWithoutMixedContentDefaults = `
schema Test:
    # A string field
    field1: str

    # A multiline field
    field2: str

    # A number field
    field3: int
`
)

func TestFixKCLSchema(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input    string
		expected string
	}{
		"no defaults": {
			input:    schemaWithNoDefaults,
			expected: schemaWithNoDefaults,
		},
		"remove simple defaults": {
			input:    schemaWithDefaults,
			expected: schemaWithNoDefaults,
		},
		"remove multiline defaults": {
			input:    schemaWithMultilineDefaults,
			expected: expectedWithoutMultilineDefaults,
		},
		"remove union type defaults": {
			input:    schemaWithUnionTypes,
			expected: expectedWithoutUnionTypeDefaults,
		},
		"remove mixed content defaults": {
			input:    schemaWithMixedContent,
			expected: expectedWithoutMixedContentDefaults,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := kclgen.FixKCLSchema(tc.input, true)
			assert.Equal(t, tc.expected, result)
		})
	}
}
