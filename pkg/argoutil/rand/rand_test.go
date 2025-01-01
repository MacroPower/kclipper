package rand

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRandString(t *testing.T) {
	ss, err := RandString(10)
	assert.NoError(t, err)
	assert.Len(t, ss, 10)
	ss, err = RandString(5)
	assert.NoError(t, err)
	assert.Len(t, ss, 5)
}
