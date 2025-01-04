package helmutil_test

import (
	"errors"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/helmutil"
)

func TestChartPkg_Set(t *testing.T) {
	t.Parallel()

	// Setup test data
	basePath := "testdata/got/set"
	chartPath := path.Join(basePath, "charts")
	_ = os.RemoveAll(chartPath)
	err := os.MkdirAll(chartPath, 0o755)
	require.NoError(t, err)

	ca := helmutil.NewChartPkg(chartPath, nil)

	tests := map[string]struct {
		chart             string
		keyValueOverrides string
		expectedError     error
	}{
		"empty chart name": {
			chart:             "",
			keyValueOverrides: "key=value",
			expectedError:     errors.New("chart name cannot be empty"),
		},
		"invalid key-value pair": {
			chart:             "test-chart",
			keyValueOverrides: "invalidpair",
			expectedError:     errors.New("no key=value pair found in 'invalidpair'"),
		},
		"invalid chart configuration attribute": {
			chart:             "test-chart",
			keyValueOverrides: "InvalidKey=value",
			expectedError:     errors.New("key 'InvalidKey' is not a valid chart configuration attribute"),
		},
		"successful set": {
			chart:             "test-chart",
			keyValueOverrides: "schemaPath=https://example.com",
			expectedError:     nil,
		},
		"successful base set": {
			chart:             "test-chart",
			keyValueOverrides: "repoURL=https://example.com",
			expectedError:     nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := ca.Set(tc.chart, tc.keyValueOverrides)
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
