package jsonschema_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/jsonschema"
)

func TestNoGenerator(t *testing.T) {
	t.Parallel()

	generator := jsonschema.DefaultNoGenerator

	// Test FromPaths.
	out, err := generator.FromPaths("path")
	require.NoError(t, err)
	require.JSONEq(t, jsonschema.EmptySchema, string(out))

	// Test FromData.
	out, err = generator.FromData([]byte("data"))
	require.NoError(t, err)
	require.JSONEq(t, jsonschema.EmptySchema, string(out))
}
