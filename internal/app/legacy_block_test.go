package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/licoy/quosparr/internal/l10n"
)

func TestLegacyBlockQuitsPreviousAppOnlyWhenAsked(t *testing.T) {
	loc := l10n.New(l10n.English)
	var asked []mygo.MessageOptions
	answer := func(button int) func(mygo.MessageOptions) int {
		return func(opts mygo.MessageOptions) int { asked = append(asked, opts); return button }
	}
	terminated, released := 0, 0
	previous := func(quits, stops bool) runningApp {
		return runningApp{
			bundleID:  "com.github.codex-runway",
			terminate: func() bool { terminated++; return quits },
			stopped:   func() bool { return stops },
			release:   func() { released++ },
		}
	}

	if resolveLegacyBlock(loc, nil, answer(0), time.Millisecond) || len(asked) != 1 || len(asked[0].Buttons) != 1 {
		t.Fatal("no running app must only explain how to quit it by hand")
	}

	asked = nil
	if resolveLegacyBlock(loc, []runningApp{previous(true, true)}, answer(1), time.Millisecond) || terminated != 0 || released != 1 {
		t.Fatal("cancel must leave the previous app running and release it")
	}
	if got := asked[0]; len(got.Buttons) != 2 || got.CancelButton != 1 {
		t.Fatalf("offer buttons = %v, cancel %d", got.Buttons, got.CancelButton)
	}

	asked = nil
	if !resolveLegacyBlock(loc, []runningApp{previous(true, true)}, answer(0), time.Millisecond) || terminated != 1 || len(asked) != 1 {
		t.Fatal("a quit previous app must let Quosparr start without another dialog")
	}

	for _, app := range []runningApp{previous(false, false), previous(true, false)} {
		asked = nil
		if resolveLegacyBlock(loc, []runningApp{app}, answer(0), time.Millisecond) || len(asked) != 2 || asked[1].Message != tx(loc, xLegacyQuitFailed) {
			t.Fatal("a refused or slow quit must be reported and keep Quosparr from starting")
		}
	}
}
