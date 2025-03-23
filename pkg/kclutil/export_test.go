package kclutil_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/kclutil"
)

func TestExporter_ExportSchemaToJSON(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		wantErr    error
		validate   func(t *testing.T, data []byte)
		pkgPath    string
		schemaName string
	}{
		"schema_not_found": {
			pkgPath:    "testdata/export",
			schemaName: "NonExistentSchema",
			wantErr:    kclutil.ErrSchemaNotFound,
		},
		"successful_export": {
			pkgPath:    "testdata/export/submod",
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

			data, err := kclutil.Export.KCLSchemaToJSONSchema(tc.pkgPath, tc.schemaName)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)

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
