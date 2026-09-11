package health_test

import (
	"context"
	"errors"
	"testing"

	"oriva/backend-go/services/health"

	"github.com/stretchr/testify/assert"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	assert.True(t, health.NewService(nil, fakePinger{nil}).Health(context.Background()))
	assert.False(t, health.NewService(nil, fakePinger{errors.New("down")}).Health(context.Background()))
}
