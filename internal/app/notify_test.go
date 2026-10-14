package app

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/egoist/mygo"
	"github.com/licoy/quosparr/internal/l10n"
)

type fakeNotify struct {
	supported bool
	errs      map[string]error
	shown     []string
}

func (f *fakeNotify) Supported() bool { return f.supported }
func (f *fakeNotify) Show(n Notification) error {
	f.shown = append(f.shown, n.ID)
	return f.errs[n.ID]
}

func TestNotifierDelivers(t *testing.T) {
	f := &fakeNotify{supported: true}
	out := (&Notifier{backend: f}).Deliver([]Notification{{ID: "a"}, {ID: "b"}})
	if !slices.Equal(Delivered(out), []string{"a", "b"}) {
		t.Errorf("outcomes %+v", out)
	}
}

func TestNotifierUnsupportedShowsNothing(t *testing.T) {
	f := &fakeNotify{}
	out := (&Notifier{backend: f}).Deliver([]Notification{{ID: "a"}})
	if out[0].Result != NotifyUnsupported || len(f.shown) != 0 || len(Delivered(out)) != 0 {
		t.Errorf("outcomes %+v shown %v", out, f.shown)
	}
}

func TestNotifierDenialStopsAskingForTheRest(t *testing.T) {
	// macOS prompts on the first notification; after a refusal the rest must
	// not each ask again, and none counts as delivered.
	f := &fakeNotify{supported: true, errs: map[string]error{"a": mygo.ErrNotificationsDenied}}
	out := (&Notifier{backend: f}).Deliver([]Notification{{ID: "a"}, {ID: "b"}, {ID: "c"}})
	for _, o := range out {
		if o.Result != NotifyDenied {
			t.Errorf("%s: %v", o.ID, o.Result)
		}
	}
	if !slices.Equal(f.shown, []string{"a"}) {
		t.Errorf("asked for %v", f.shown)
	}
}

func TestNotifierOneFailureDoesNotStopTheOthers(t *testing.T) {
	f := &fakeNotify{supported: true, errs: map[string]error{"a": errors.New("no service")}}
	out := (&Notifier{backend: f}).Deliver([]Notification{{ID: "a"}, {ID: "b"}})
	if out[0].Result != NotifyFailed || out[1].Result != NotifyDelivered {
		t.Errorf("outcomes %+v", out)
	}
}

func TestNotifierSerializesPrompts(t *testing.T) {
	f := &fakeNotify{supported: true}
	n := &Notifier{backend: f}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); n.Deliver([]Notification{{ID: "x"}}) }()
	}
	wg.Wait()
	if len(f.shown) != 8 {
		t.Errorf("shown %d", len(f.shown))
	}
}

func TestNotifyProblemTexts(t *testing.T) {
	l := l10n.New(l10n.English)
	if notifyProblem(l, Outcome{Result: NotifyDelivered}) != "" {
		t.Error("a delivered notification has no problem")
	}
	for r, want := range map[NotifyResult]string{
		NotifyUnsupported: "not available", NotifyDenied: "turned off", NotifyFailed: "boom",
	} {
		got := notifyProblem(l, Outcome{Result: r, Err: errors.New("boom")})
		if !strings.Contains(got, want) {
			t.Errorf("%v: %q lacks %q", r, got, want)
		}
	}
}

func TestTestActionReturnsAtOnceAndExplainsRefusals(t *testing.T) {
	f := &fakeNotify{supported: true, errs: map[string]error{"quosparr-test": mygo.ErrNotificationsDenied}}
	done := make(chan string, 1)
	act := (&Notifier{backend: f}).TestAction(
		func() l10n.Localizer { return l10n.New(l10n.English) },
		func(title, msg string) { done <- msg },
	)
	if got := act(); !strings.Contains(got, "requested") {
		t.Errorf("immediate text %q", got)
	}
	if msg := <-done; !strings.Contains(msg, "turned off") {
		t.Errorf("explanation %q", msg)
	}
}
