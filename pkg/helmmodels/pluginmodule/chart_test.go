package pluginmodule_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/helmmodels/pluginmodule"
)

func TestGenerateHelmModule(t *testing.T) {
	t.Parallel()

	err := os.Chdir("../../../")
	require.NoError(t, err)

	b := &bytes.Buffer{}

	cb := pluginmodule.ChartBase{}
	err = cb.GenerateKCL(b)
	require.NoError(t, err)
	assert.NotEmpty(t, b.String())
	// assert.Equal(t, "", b.String())

	b.Truncate(0)
	cc := pluginmodule.ChartConfig{}
	err = cc.GenerateKCL(b)
	require.NoError(t, err)
	assert.NotEmpty(t, b.String())
	// assert.Equal(t, "", b.String())

	b.Truncate(0)
	c := pluginmodule.Chart{}
	err = c.GenerateKCL(b)
	require.NoError(t, err)
	assert.NotEmpty(t, b.String())
	// assert.Equal(t, "", b.String())
}
