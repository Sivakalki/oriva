package helpers_test

import (
	"testing"

	"oriva/backend-go/utils/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndVerify(t *testing.T) {
	h, err := helpers.HashPassword("s3cr3t", 10)
	require.NoError(t, err)

	assert.True(t, helpers.VerifyPassword("s3cr3t", h))
	assert.False(t, helpers.VerifyPassword("wrong", h))
}

func TestHashPasswordClampsCost(t *testing.T) {
	h, err := helpers.HashPassword("pw", 99)
	require.NoError(t, err)
	assert.True(t, helpers.VerifyPassword("pw", h))
}
