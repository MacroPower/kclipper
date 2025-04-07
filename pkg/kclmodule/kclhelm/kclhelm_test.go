package kclhelm_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/kclmodule/kclhelm"
)

//nolint:paralleltest // Due to t.Chdir.
func TestGenerateHelmModule(t *testing.T) {
	t.Chdir("../../../")

	b := &bytes.Buffer{}

	cb := kclhelm.ChartBase{}
	err := cb.GenerateKCL(b)
	require.NoError(t, err)
	assert.NotEmpty(t, b.String())
	// assert.Equal(t, "", b.String())

	b.Truncate(0)

	cc := kclhelm.ChartConfig{}
	err = cc.GenerateKCL(b)
	require.NoError(t, err)
	assert.NotEmpty(t, b.String())
	// assert.Equal(t, "", b.String())

	b.Truncate(0)

	cr := kclhelm.ChartRepo{}
	err = cr.GenerateKCL(b)
	require.NoError(t, err)
	assert.NotEmpty(t, b.String())
	// assert.Equal(t, "", b.String())

	b.Truncate(0)

	c := kclhelm.Chart{}
	err = c.GenerateKCL(b)
	require.NoError(t, err)
	assert.NotEmpty(t, b.String())
	// assert.Equal(t, "", b.String())

	b.Truncate(0)
}
