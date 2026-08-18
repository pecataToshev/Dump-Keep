package notify

import "errors"

// FanOut fans out a message to multiple notifiers. All notifiers are
// called regardless of individual failures; errors are joined.
type FanOut struct {
	notifiers []Notifier
}

// NewFanOut creates an empty composite notifier.
func NewFanOut() FanOut {
	return FanOut{}
}

// Add appends a notifier. Nil values are ignored.
func (m *FanOut) Add(n Notifier) {
	if n != nil {
		m.notifiers = append(m.notifiers, n)
	}
}

// Notify sends the message to all configured notifiers. Returns a joined
// error if any notifier fails; nil if all succeed (or if empty).
func (m FanOut) Notify(message string) error {
	var errs []error
	for _, n := range m.notifiers {
		if err := n.Notify(message); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Types returns the type names of all configured notifiers.
// Returns ["none"] when no notifiers are configured.
func (m FanOut) Types() []string {
	if len(m.notifiers) == 0 {
		return []string{"none"}
	}
	types := make([]string, len(m.notifiers))
	for i, n := range m.notifiers {
		types[i] = n.Type()
	}
	return types
}
