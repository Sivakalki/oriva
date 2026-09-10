package health

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	assert.True(t, NewService(nil, fakePinger{nil}).Health(context.Background()))
	assert.False(t, NewService(nil, fakePinger{errors.New("down")}).Health(context.Background()))
}
