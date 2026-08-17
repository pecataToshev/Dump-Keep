package notify

import (
	"errors"
	"testing"
)

type stubNotifier struct {
	msgs   []string
	err    error
	called bool
}

func (s *stubNotifier) Notify(message string) error {
	s.called = true
	s.msgs = append(s.msgs, message)
	return s.err
}

func TestMulti_NoNotifiers(t *testing.T) {
	m := NewMulti()
	if err := m.Notify("hello"); err != nil {
		t.Errorf("empty Multi should return nil, got %v", err)
	}
}

func TestMulti_FanOut(t *testing.T) {
	a := &stubNotifier{}
	b := &stubNotifier{}
	m := NewMulti(a, b)

	if err := m.Notify("test message"); err != nil {
		t.Fatalf("Notify failed: %v", err)
	}

	if !a.called || !b.called {
		t.Error("expected both notifiers to be called")
	}
	if len(a.msgs) != 1 || a.msgs[0] != "test message" {
		t.Errorf("notifier A got %v, want ['test message']", a.msgs)
	}
	if len(b.msgs) != 1 || b.msgs[0] != "test message" {
		t.Errorf("notifier B got %v, want ['test message']", b.msgs)
	}
}

func TestMulti_PartialFailure(t *testing.T) {
	good := &stubNotifier{}
	bad := &stubNotifier{err: errors.New("webhook down")}
	m := NewMulti(good, bad)

	err := m.Notify("test")
	if err == nil {
		t.Fatal("expected error when one notifier fails, got nil")
	}

	// Good notifier should still have been called
	if !good.called {
		t.Error("good notifier should still be called even if bad one fails")
	}
	// Bad notifier should also have been called
	if !bad.called {
		t.Error("bad notifier should have been called")
	}
}

func TestMulti_AllFail(t *testing.T) {
	a := &stubNotifier{err: errors.New("error A")}
	b := &stubNotifier{err: errors.New("error B")}
	m := NewMulti(a, b)

	err := m.Notify("test")
	if err == nil {
		t.Fatal("expected error when all notifiers fail, got nil")
	}
}

func TestMulti_FiltersNil(t *testing.T) {
	good := &stubNotifier{}
	m := NewMulti(nil, good, nil)

	if err := m.Notify("test"); err != nil {
		t.Fatalf("Notify failed: %v", err)
	}
	if !good.called {
		t.Error("non-nil notifier should be called")
	}
}

func TestNoop(t *testing.T) {
	n := Noop{}
	if err := n.Notify("anything"); err != nil {
		t.Errorf("Noop should never error, got %v", err)
	}
}
