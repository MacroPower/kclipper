package version_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/version"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, version.Revision)
}
