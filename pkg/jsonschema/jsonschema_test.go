package jsonschema_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

var testDataDir string

func init() {
	//nolint:dogsled
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testDataDir = filepath.Join(dir, "testdata")
}

func TestGetGenerator(t *testing.T) {
	t.Parallel()

	require.IsType(t, jsonschema.DefaultAutoGenerator, jsonschema.GetGenerator(jsonschema.AutoGeneratorType))
	require.IsType(t, jsonschema.DefaultNoGenerator, jsonschema.GetGenerator("UNKNOWN"))
}
