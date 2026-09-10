// Package notify sends outbound email (candidate interview invites). The "log"
// transport just logs the rendered message and is the dev default; "smtp" sends
// for real (docs/CLAUDE.md: prefer free/OSS while experimenting).
package notify

import (
	"context"
	"fmt"
	"net/smtp"
	"time"

	"oriva/backend-go/config"

	"go.uber.org/zap"
)

// Email is one outbound message.
type Email struct {
	To      string
	Subject string
	Text    string
}

// Sender delivers an Email.
type Sender interface {
	Send(ctx context.Context, e Email) error
}

// NewSender builds the Sender for the configured transport.
func NewSender(cfg config.Notify, logger *zap.Logger) (Sender, error) {
	switch cfg.Transport {
	case "log", "":
		return &logSender{from: cfg.From, logger: logger}, nil
	case "smtp":
		if cfg.SMTP.Host == "" {
			return nil, fmt.Errorf("notify: smtp transport needs notify.smtp.host")
		}
		return &smtpSender{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("notify: unknown transport %q", cfg.Transport)
	}
}

type logSender struct {
	from   string
	logger *zap.Logger
}

func (s *logSender) Send(_ context.Context, e Email) error {
	s.logger.Info("email (log transport)",
		zap.String("from", s.from),
		zap.String("to", e.To),
		zap.String("subject", e.Subject),
		zap.String("body", e.Text),
	)
	return nil
}

type smtpSender struct{ cfg config.Notify }

func (s *smtpSender) Send(_ context.Context, e Email) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTP.Host, s.cfg.SMTP.Port)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		s.cfg.From, e.To, e.Subject, e.Text)
	var auth smtp.Auth
	if s.cfg.SMTP.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTP.Username, s.cfg.SMTP.Password, s.cfg.SMTP.Host)
	}
	return smtp.SendMail(addr, auth, s.cfg.From, []string{e.To}, []byte(msg))
}

// InviteEmail renders the candidate invite.
func InviteEmail(to, candidateName, jobTitle, joinURL string, scheduledAt time.Time) Email {
	when := scheduledAt.UTC().Format("Monday, 2 Jan 2006 15:04 MST")
	name := candidateName
	if name == "" {
		name = "there"
	}
	body := fmt.Sprintf(
		"Hi %s,\n\n"+
			"You've been invited to an interview for %s.\n\n"+
			"When: %s\n"+
			"Join here: %s\n\n"+
			"Open the link any time — the page will let you in at the scheduled time.\n",
		name, jobTitle, when, joinURL,
	)
	return Email{
		To:      to,
		Subject: fmt.Sprintf("Your interview for %s", jobTitle),
		Text:    body,
	}
}
