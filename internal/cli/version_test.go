package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/internal/cli"
)

func TestVersionCmd(t *testing.T) {
	t.Parallel()

	tc := cli.NewRootCmd("test_version", "", "")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	tc.SetArgs([]string{"version"})
	tc.SetOut(stdout)
	tc.SetErr(stderr)

	err := tc.Execute()
	require.NoError(t, err)
	assert.Regexp(t, `\d+\.\d+\.\d+`, stdout.String())
	assert.Empty(t, stderr.String())
}
