package jsonschema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

func TestConvertToKCLSchema(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		expectedErr     error
		input           []byte
		containsStrs    []string
		notContainsStrs []string
		removeDefaults  bool
	}{
		"simple schema": {
			input: []byte(`{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"name": {
						"type": "string"
					},
					"age": {
						"type": "integer",
						"default": 30
					}
				}
			}`),
			removeDefaults: false,
			expectedErr:    nil,
			containsStrs:   []string{"schema Values:", "name?: str", "age?: int = 30"},
		},
		"schema with removal of defaults": {
			input: []byte(`{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"name": {
						"type": "string"
					},
					"age": {
						"type": "integer",
						"default": 30
					}
				}
			}`),
			removeDefaults:  true,
			expectedErr:     nil,
			containsStrs:    []string{"schema Values:", "name?: str", "age?: int"},
			notContainsStrs: []string{"age?: int = 30"}, // Verify default was removed
		},
		"invalid json": {
			input:          []byte(`not a valid json schema`),
			removeDefaults: false,
			expectedErr:    jsonschema.ErrUnmarshalSchema,
			containsStrs:   nil,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, err := jsonschema.ConvertToKCLSchema(tc.input, tc.removeDefaults)

			if tc.expectedErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.expectedErr)

				return
			}

			require.NoError(t, err)
			resultStr := string(result)

			for _, str := range tc.containsStrs {
				assert.Contains(t, resultStr, str)
			}

			for _, str := range tc.notContainsStrs {
				assert.NotContains(t, resultStr, str)
			}
		})
	}
}

func TestConvertToKCLCompatibleJSONSchema(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		expectedErr error
		input       []byte
	}{
		"simple schema": {
			input: []byte(`{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"$id": "http://example.com/schema.json",
				"type": "object",
				"properties": {
					"name": {
						"type": "string"
					}
				}
			}`),
			expectedErr: nil,
		},
		"invalid json": {
			input:       []byte(`not a valid json schema`),
			expectedErr: jsonschema.ErrUnmarshalSchema,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, err := jsonschema.ConvertToKCLCompatibleJSONSchema(tc.input)

			if tc.expectedErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.expectedErr)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// The ID should be removed
			assert.NotContains(t, string(result), `"$id":`)
		})
	}
}
