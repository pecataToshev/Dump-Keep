// Package notify delivers operational messages (failures, heartbeats)
// to one or more messaging channels.
package notify

import (
	"errors"
	"regexp"
)

// Notifier delivers a message to a channel.
type Notifier interface {
	Notify(message string) error
}

// Noop discards all messages. Used when no channel is configured.
type Noop struct{}

func (Noop) Notify(string) error { return nil }

// Multi fans out a message to multiple notifiers. All notifiers are
// called regardless of individual failures; errors are joined.
type Multi struct {
	notifiers []Notifier
}

// NewMulti creates a composite notifier from the given notifiers.
// Nil notifiers are filtered out.
func NewMulti(notifiers ...Notifier) Multi {
	var nn []Notifier
	for _, n := range notifiers {
		if n != nil {
			nn = append(nn, n)
		}
	}
	return Multi{notifiers: nn}
}

// Notify sends the message to all configured notifiers. Returns a joined
// error if any notifier fails; nil if all succeed (or if empty).
func (m Multi) Notify(message string) error {
	var errs []error
	for _, n := range m.notifiers {
		if err := n.Notify(message); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// connection URLs may appear in error messages; never post passwords.
var urlPassword = regexp.MustCompile(`(://[^/:@\s]+:)[^@\s]+@`)

// Redact masks passwords in connection URLs embedded in s.
func Redact(s string) string {
	return urlPassword.ReplaceAllString(s, "${1}***@")
}
