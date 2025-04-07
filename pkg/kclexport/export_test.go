package kclexport_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/kclerrors"
	"github.com/MacroPower/kclipper/pkg/kclexport"
)

func TestExporter_ExportSchemaToJSON(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		err        error
		validate   func(t *testing.T, data []byte)
		pkgPath    string
		schemaName string
	}{
		"schema not found": {
			pkgPath:    "testdata/submod",
			schemaName: "NonExistentSchema",
			err:        kclerrors.ErrSchemaNotFound,
		},
		"successful export": {
			pkgPath:    "testdata/submod",
			schemaName: "Config",
			validate: func(t *testing.T, data []byte) {
				t.Helper()

				var schema map[string]any
				err := json.Unmarshal(data, &schema)
				require.NoError(t, err)
				require.Equal(t, "object", schema["type"])

				require.Contains(t, schema, "definitions")
				assert.Contains(t, schema["definitions"], "Container")
				assert.Contains(t, schema["definitions"], "Env")

				require.Contains(t, schema, "properties")
				properties, ok := schema["properties"].(map[string]any)
				require.True(t, ok, "properties should be a map")
				require.Contains(t, properties, "enabled")
				enabled, ok := properties["enabled"].(map[string]any)
				require.True(t, ok, "properties.enabled should be a map")
				require.Contains(t, enabled, "type")
				assert.Equal(t, "boolean", enabled["type"])
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data, err := kclexport.Export.KCLSchemaToJSONSchema(tc.pkgPath, tc.schemaName)
			if tc.err != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.err)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, data)
			if tc.validate != nil {
				tc.validate(t, data)
			}

			// err = os.WriteFile("testdata/export/output.json", data, 0o600)
			// require.NoError(t, err)
		})
	}
}
