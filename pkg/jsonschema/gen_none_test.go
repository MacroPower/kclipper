package jsonschema_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

func TestNoGenerator(t *testing.T) {
	t.Parallel()

	generator := jsonschema.DefaultNoGenerator

	// Test FromPaths
	_, err := generator.FromPaths("path")
	require.Error(t, err)

	// Test FromData
	_, err = generator.FromData([]byte("data"))
	require.Error(t, err)
}
