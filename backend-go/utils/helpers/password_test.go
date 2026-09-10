package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("s3cr3t", 10)
	require.NoError(t, err)

	assert.True(t, VerifyPassword("s3cr3t", h))
	assert.False(t, VerifyPassword("wrong", h))
}

func TestHashPasswordClampsCost(t *testing.T) {
	h, err := HashPassword("pw", 99)
	require.NoError(t, err)
	assert.True(t, VerifyPassword("pw", h))
}
