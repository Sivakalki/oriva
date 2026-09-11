package email_repo_test

import (
	"context"
	"testing"
	"time"

	"oriva/backend-go/config"
	"oriva/backend-go/repositories/notifications/email_repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestInviteEmail(t *testing.T) {
	at := time.Date(2026, 9, 20, 14, 30, 0, 0, time.UTC)
	e := email_repo.InviteEmail("cand@example.com", "Alice", "Backend Engineer", "http://fe/join/tok123", at)

	assert.Equal(t, "cand@example.com", e.To)
	assert.Contains(t, e.Subject, "Backend Engineer")
	assert.Contains(t, e.Text, "Alice")
	assert.Contains(t, e.Text, "Backend Engineer")
	assert.Contains(t, e.Text, "http://fe/join/tok123")
	assert.Contains(t, e.Text, "14:30")
}

func TestInviteEmail_NoName(t *testing.T) {
	e := email_repo.InviteEmail("x@y.com", "", "Role", "http://fe/join/t", time.Now())
	assert.Contains(t, e.Text, "Hi there,")
}

func TestNewSender_Log(t *testing.T) {
	s, err := email_repo.NewSender(config.Notify{Transport: "log"}, zap.NewNop())
	require.NoError(t, err)
	assert.NoError(t, s.Send(context.Background(), email_repo.Email{To: "a@b.com", Subject: "s", Text: "t"}))
}

func TestNewSender_DefaultsToLog(t *testing.T) {
	s, err := email_repo.NewSender(config.Notify{}, zap.NewNop())
	require.NoError(t, err)
	assert.NoError(t, s.Send(context.Background(), email_repo.Email{}))
}

func TestNewSender_SMTPNeedsHost(t *testing.T) {
	_, err := email_repo.NewSender(config.Notify{Transport: "smtp"}, zap.NewNop())
	assert.Error(t, err)
}

func TestNewSender_UnknownTransport(t *testing.T) {
	_, err := email_repo.NewSender(config.Notify{Transport: "carrier-pigeon"}, zap.NewNop())
	assert.Error(t, err)
}
