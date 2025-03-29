package jsonschema_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

func TestReflector(t *testing.T) {
	t.Parallel()

	// Define test types to use with reflection
	type Person struct {
		Name  string   `json:"name"`
		Email string   `json:"email,omitempty"`
		Tags  []string `json:"tags,omitempty"`
		Age   int      `json:"age"`
	}

	type User struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Status    string `json:"status"`
	}

	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create a schema file that we'll reference
	refSchemaContent := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"name": {
				"type": "string"
			},
			"age": {
				"type": "integer"
			}
		}
	}`
	refSchemaPath := filepath.Join(tmpDir, "ref-schema.json")
	err := os.WriteFile(refSchemaPath, []byte(refSchemaContent), 0o666)
	require.NoError(t, err)

	// Test replacement function with regexp pattern
	t.Run("Replace", func(t *testing.T) {
		t.Parallel()

		// Create a simple reflected schema to test with
		reflector := jsonschema.NewReflector()
		require.NotNil(t, reflector)

		schema := reflector.Reflect(reflect.TypeOf(Person{}))
		require.NotNil(t, schema)

		// Create a buffer to hold the output
		var buf strings.Builder

		// Apply the replacement and generate KCL
		pattern := regexp.MustCompile(`(name:)`)
		replaceOpt := jsonschema.Replace(pattern, "userName$1")

		err := schema.GenerateKCL(&buf, replaceOpt)
		require.NoError(t, err)

		// Verify the replacement was applied
		result := buf.String()
		assert.Contains(t, result, "userNamename:")
	})

	// Test WithType option
	t.Run("WithType", func(t *testing.T) {
		t.Parallel()

		// Create a simple reflected schema
		reflector := jsonschema.NewReflector()
		require.NotNil(t, reflector)

		schema := reflector.Reflect(reflect.TypeOf(Person{}))
		require.NotNil(t, schema)

		// Set a property with WithType
		schema.SetProperty("name", jsonschema.WithType("string"))

		// Verify the modification through KCL generation
		var buf strings.Builder
		err := schema.GenerateKCL(&buf)
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "name: str")
	})

	// Test WithEnum option
	t.Run("WithEnum", func(t *testing.T) {
		t.Parallel()

		// Create a simple reflected schema
		reflector := jsonschema.NewReflector()
		require.NotNil(t, reflector)

		schema := reflector.Reflect(reflect.TypeOf(User{}))
		require.NotNil(t, schema)

		// Set enum values
		enumValues := []any{"active", "inactive"}
		schema.SetProperty("status", jsonschema.WithEnum(enumValues))

		// Verify through KCL generation
		var buf strings.Builder
		err := schema.GenerateKCL(&buf)
		require.NoError(t, err)

		// The exact string format depends on the KCL generator, but should contain the enum values
		result := buf.String()
		assert.Contains(t, result, "status:")
		assert.Contains(t, result, "active")
		assert.Contains(t, result, "inactive")
	})

	// Test WithNoItems option
	t.Run("WithNoItems", func(t *testing.T) {
		t.Parallel()

		// Create a reflector
		reflector := jsonschema.NewReflector()
		require.NotNil(t, reflector)

		// Create a schema with an array property
		schema := reflector.Reflect(reflect.TypeOf(Person{}))
		require.NotNil(t, schema)

		// Get the original KCL before applying WithNoItems
		var bufBefore strings.Builder
		err := schema.GenerateKCL(&bufBefore)
		require.NoError(t, err)
		before := bufBefore.String()

		// Apply WithNoItems and check the result
		schema.SetProperty("tags", jsonschema.WithNoItems())

		var bufAfter strings.Builder
		err = schema.GenerateKCL(&bufAfter)
		require.NoError(t, err)

		// Since WithNoItems removes the Items definition, the output would be different
		// This is an indirect way to test WithNoItems worked
		assert.NotEqual(t, before, bufAfter.String())
	})

	// Test SetProperty and RemoveProperty
	t.Run("PropertyModifications", func(t *testing.T) {
		t.Parallel()

		// Create a simple reflected schema
		reflector := jsonschema.NewReflector()
		require.NotNil(t, reflector)

		schema := reflector.Reflect(reflect.TypeOf(Person{}))
		require.NotNil(t, schema)

		// Remove a property
		schema.RemoveProperty("email")

		// Verify property was removed
		var buf strings.Builder
		err := schema.GenerateKCL(&buf)
		require.NoError(t, err)

		result := buf.String()
		assert.Contains(t, result, "name:")
		assert.NotContains(t, result, "email:")

		// Test SetOrRemoveProperty to remove
		schema = reflector.Reflect(reflect.TypeOf(User{}))
		require.NotNil(t, schema)

		schema.SetOrRemoveProperty("lastName", false)

		buf.Reset()
		err = schema.GenerateKCL(&buf)
		require.NoError(t, err)

		result = buf.String()
		assert.Contains(t, result, "firstName:")
		assert.NotContains(t, result, "lastName:")

		// Test SetOrRemoveProperty to set
		schema.SetOrRemoveProperty("firstName", true, jsonschema.WithType("string"))

		buf.Reset()
		err = schema.GenerateKCL(&buf)
		require.NoError(t, err)

		result = buf.String()
		assert.Contains(t, result, "firstName: str")
	})
}
