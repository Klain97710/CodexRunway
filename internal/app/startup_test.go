package app

import (
	"errors"
	"slices"
	"testing"
)

func TestRunStepsOrderAndSuccess(t *testing.T) {
	var order []string
	step := func(name string) func() error {
		return func() error { order = append(order, name); return nil }
	}
	r := Seams{
		LoadConfig:      step("config"),
		MigrateLegacy:   step("migrate"),
		VerifyProxy:     step("proxy"),
		ReconcileSystem: step("system"),
	}.RunSteps()
	if want := []string{"config", "migrate", "proxy", "system"}; !slices.Equal(order, want) {
		t.Fatalf("order %v, want %v", order, want)
	}
	if len(r.Problems) != 0 || !r.BackgroundAllowed {
		t.Fatalf("unexpected report %+v", r)
	}
}

func TestRunStepsNilSeamsAreSkipped(t *testing.T) {
	r := Seams{}.RunSteps()
	if len(r.Problems) != 0 || !r.BackgroundAllowed {
		t.Fatalf("unexpected report %+v", r)
	}
}

func TestBrokenConfigSkipsMigrationAndBlocksBackground(t *testing.T) {
	migrated := false
	r := Seams{
		LoadConfig:    func() error { return errors.New("bad json") },
		MigrateLegacy: func() error { migrated = true; return nil },
	}.RunSteps()
	if migrated {
		t.Error("migration must not run on an unreadable config")
	}
	if r.BackgroundAllowed || len(r.Problems) != 1 || r.Problems[0].Step != "config" {
		t.Fatalf("unexpected report %+v", r)
	}
}

func TestBrokenProxyBlocksBackgroundButSystemStateStillRuns(t *testing.T) {
	ran := false
	r := Seams{
		VerifyProxy:     func() error { return errors.New("credentials missing") },
		ReconcileSystem: func() error { ran = true; return errors.New("x") },
	}.RunSteps()
	if !ran || r.BackgroundAllowed || len(r.Problems) != 2 {
		t.Fatalf("unexpected report %+v ran=%v", r, ran)
	}
}
