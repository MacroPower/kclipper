package schema_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/schema"
)

func TestNoGenerator(t *testing.T) {
	t.Parallel()

	generator := schema.DefaultNoGenerator

	// Test FromPaths.
	out, err := generator.FromPaths("path")
	require.NoError(t, err)
	require.JSONEq(t, schema.EmptySchema, string(out))

	// Test FromData.
	out, err = generator.FromData([]byte("data"))
	require.NoError(t, err)
	require.JSONEq(t, schema.EmptySchema, string(out))
}
