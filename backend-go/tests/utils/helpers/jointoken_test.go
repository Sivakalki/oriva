package helpers_test

import (
	"regexp"
	"testing"

	"oriva/backend-go/utils/helpers"

	"github.com/stretchr/testify/assert"
)

var urlSafe = regexp.MustCompile(`^[A-Za-z0-9_-]{24}$`)

func TestNewJoinToken(t *testing.T) {
	a, b := helpers.NewJoinToken(), helpers.NewJoinToken()
	assert.Regexp(t, urlSafe, a)
	assert.Regexp(t, urlSafe, b)
	assert.NotEqual(t, a, b)
}
