package jointoken

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var urlSafe = regexp.MustCompile(`^[A-Za-z0-9_-]{24}$`)

func TestNew(t *testing.T) {
	a, b := New(), New()
	assert.Regexp(t, urlSafe, a)
	assert.Regexp(t, urlSafe, b)
	assert.NotEqual(t, a, b)
}
