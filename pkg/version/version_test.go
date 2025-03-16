package version_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/version"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, version.Revision)
}
