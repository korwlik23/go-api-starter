// Package mail defines the provider-neutral outbound mail boundary. Identity
// use cases depend on Sender instead of SMTP details, so a project can select
// SMTP, a local file sink, or a queue-backed provider at composition time.
package mail

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrInvalidMessage = errors.New("mail: invalid message")
	ErrInvalidSender  = errors.New("mail: invalid sender")
)

type Message struct {
	To      string
	Subject string
	Text    string
}

func (message Message) Validate() error {
	address, err := mail.ParseAddress(strings.TrimSpace(message.To))
	if err != nil || address.Address != strings.TrimSpace(message.To) || strings.TrimSpace(message.Subject) == "" || strings.TrimSpace(message.Text) == "" || len(message.Subject) > 255 || len(message.Text) > 1<<20 {
		return ErrInvalidMessage
	}
	return nil
}

type Sender interface {
	Send(context.Context, Message) error
}

type FileSender struct{ Root string }

func NewFileSender(root string) (*FileSender, error) {
	root = strings.TrimSpace(root)
	if root == "" || filepath.IsAbs(root) == false {
		return nil, ErrInvalidSender
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, ErrInvalidSender
	}
	return &FileSender{Root: root}, nil
}

func (sender *FileSender) Send(ctx context.Context, message Message) error {
	if sender == nil || strings.TrimSpace(sender.Root) == "" || ctx == nil {
		return ErrInvalidSender
	}
	if err := message.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	recipientHash := sha256.Sum256([]byte(strings.TrimSpace(message.To)))
	filename := fmt.Sprintf("message-%s-%d.eml", hex.EncodeToString(recipientHash[:8]), time.Now().UTC().UnixNano())
	path := filepath.Join(sender.Root, filename)
	contents := "To: " + message.To + "\r\nSubject: " + message.Subject + "\r\n\r\n" + message.Text + "\r\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return fmt.Errorf("mail: write local message: %w", err)
	}
	return nil
}

type NoopSender struct{}

func (NoopSender) Send(ctx context.Context, message Message) error {
	if ctx == nil {
		return ErrInvalidSender
	}
	return message.Validate()
}

var _ Sender = (*FileSender)(nil)
var _ Sender = NoopSender{}
