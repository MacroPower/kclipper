package chartmodule_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/helmmodels/chartmodule"
)

func TestGenerateChart(t *testing.T) {
	t.Parallel()

	b := &bytes.Buffer{}

	cc := chartmodule.ChartConfig{}
	err := cc.GenerateKCL(b)
	require.NoError(t, err)
	require.NotEmpty(t, b.String())
	// assert.Equal(t, "", b.String())

	b.Truncate(0)
	c := chartmodule.Chart{}
	err = c.GenerateKCL(b)
	require.NoError(t, err)
	require.NotEmpty(t, b.String())
	// assert.Equal(t, "", b.String())
}
