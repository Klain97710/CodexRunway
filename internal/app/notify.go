package app

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/egoist/mygo"
	"github.com/licoy/quosparr/internal/l10n"
)

// Notification is one desktop notification. ID repeats for the same alert
// (alerts.Alert.ID), so showing it again replaces the earlier one.
type Notification struct {
	ID    string
	Title string
	Body  string
}

// NotifyResult is what happened to one notification.
type NotifyResult int

const (
	// NotifyDelivered: the system has the notification.
	NotifyDelivered NotifyResult = iota
	// NotifyUnsupported: this run cannot show notifications (an unpackaged
	// macOS run, or no notification service on Linux).
	NotifyUnsupported
	// NotifyDenied: the user has not allowed notifications (macOS asks on
	// the first one; later denials are made in System Settings).
	NotifyDenied
	// NotifyFailed: the system refused it for another reason.
	NotifyFailed
)

// Outcome is the result for the notification with ID.
type Outcome struct {
	ID     string
	Result NotifyResult
	Err    error
}

// notifyBackend is the part of mygo that shows notifications.
type notifyBackend interface {
	Supported() bool
	Show(n Notification) error
}

type mygoNotifyBackend struct{}

func (mygoNotifyBackend) Supported() bool { return mygo.NotificationsSupported() }

func (mygoNotifyBackend) Show(n Notification) error {
	// Show waits for the system's answer, which on macOS includes the
	// permission prompt of the first notification.
	return mygo.NewNotification(mygo.NotificationOptions{
		ID: n.ID, Title: n.Title, Body: n.Body,
	}).Show()
}

// Notifier delivers notifications. Permissions, by platform:
//
//   - macOS asks once, on the first notification, and Show waits for the
//     answer; a refusal comes back as ErrNotificationsDenied from then on
//     (until the user allows it in System Settings). Needs the packaged app.
//   - Windows shows notification-area balloons, which need no permission.
//   - Linux uses the desktop's notification service; without one Show fails.
//
// Deliver blocks until the system answers, so call it from a goroutine, never
// from the main thread's view code.
type Notifier struct {
	backend notifyBackend

	mu sync.Mutex // one prompt at a time
}

// NewNotifier is the Notifier of the running app.
func NewNotifier() *Notifier { return &Notifier{backend: mygoNotifyBackend{}} }

// Deliver shows msgs in order and reports each one. After the first denial
// the rest are reported denied without asking again; after "unsupported"
// likewise.
func (n *Notifier) Deliver(msgs []Notification) []Outcome {
	out := make([]Outcome, 0, len(msgs))
	if len(msgs) == 0 {
		return out
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.backend.Supported() {
		for _, m := range msgs {
			out = append(out, Outcome{ID: m.ID, Result: NotifyUnsupported})
		}
		return out
	}
	stop := NotifyDelivered
	for _, m := range msgs {
		if stop != NotifyDelivered {
			out = append(out, Outcome{ID: m.ID, Result: stop})
			continue
		}
		o := Outcome{ID: m.ID}
		if err := n.backend.Show(m); err != nil {
			o.Err = err
			o.Result = NotifyFailed
			if errors.Is(err, mygo.ErrNotificationsDenied) {
				o.Result, stop = NotifyDenied, NotifyDenied
			}
		}
		out = append(out, o)
	}
	return out
}

// Delivered returns the IDs that reached the system.
func Delivered(outcomes []Outcome) []string {
	var ids []string
	for _, o := range outcomes {
		if o.Result == NotifyDelivered {
			ids = append(ids, o.ID)
		}
	}
	return ids
}

// TestAction is the settings button "Test notification". It returns the
// message to show at once and delivers in the background; a refusal is then
// explained in a dialog (the system's own prompt appears first on macOS).
func (n *Notifier) TestAction(loc func() l10n.Localizer, explain func(title, message string)) func() string {
	return func() string {
		l := loc()
		go func() {
			out := n.Deliver([]Notification{{
				ID:    "quosparr-test",
				Title: l.Text(l10n.KeyTestNotificationTitle),
				Body:  l.Text(l10n.KeyTestNotificationBody),
			}})
			if msg := notifyProblem(l, out[0]); msg != "" && explain != nil {
				explain(l.Text(l10n.KeyTestNotification), msg)
			}
		}()
		return tx(l, xNotifyRequested)
	}
}

// notifyProblem is the user-facing explanation of a failed delivery, or "".
func notifyProblem(l l10n.Localizer, o Outcome) string {
	switch o.Result {
	case NotifyUnsupported:
		return tx(l, xNotifyUnsupported)
	case NotifyDenied:
		return tx(l, xNotifyDenied)
	case NotifyFailed:
		log.Println("notification:", o.Err)
		return tx(l, xNotifyFailed, fmt.Sprint(o.Err))
	}
	return ""
}

// ShowDialog is an explain func for TestAction: a modal message. Call it
// off the main thread (TestAction does).
func ShowDialog(title, message string) {
	_, _ = mygo.Dialog.Message(mygo.MessageOptions{
		Type: mygo.MessageInfo, Message: title, Detail: message,
	})
}
