// Package notify delivers operational messages (failures, heartbeats)
// to one or more messaging channels.
package notify

import "regexp"

// Notifier delivers a message to a channel.
type Notifier interface {
	Notify(message string) error
	Type() string
}

// connection URLs may appear in error messages; never post passwords.
var urlPassword = regexp.MustCompile(`(://[^/:@\s]+:)[^@\s]+@`)

// Redact masks passwords in connection URLs embedded in s.
func Redact(s string) string {
	return urlPassword.ReplaceAllString(s, "${1}***@")
}
