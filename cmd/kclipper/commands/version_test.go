package commands_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/cmd/kclipper/commands"
)

func TestVersionCmd(t *testing.T) {
	tc := commands.NewRootCmd("test_version", "", "")
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
