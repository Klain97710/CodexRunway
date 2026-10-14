package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/l10n"
)

func TestWidgetConfigurationsDriveRequirementsAndQueryFailureIsVisible(t *testing.T) {
	w := newWidgetCoordinator(t.TempDir(), false)
	defer w.close()
	reqs, failures := make(chan export.Requirements, 1), make(chan error, 1)
	w.onRequirements = func(r export.Requirements) { reqs <- r }
	w.onError = func(err error) { failures <- err }
	w.native = &widgetNative{configurations: func(context.Context) ([]widgetConfiguration, error) {
		return []widgetConfiguration{{export.KindOverview, export.FamilySmall}, {export.KindResetToday, export.FamilyMedium}, {"unknown", export.FamilyLarge}}, nil
	}}
	w.refresh()
	select {
	case r := <-reqs:
		if r != export.RequireProviderQuota|export.RequireResetToday {
			t.Fatalf("requirements: %v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("requirements not delivered")
	}
	w.wg.Wait()
	w.native.configurations = func(context.Context) ([]widgetConfiguration, error) { return nil, errors.New("query denied") }
	w.refresh()
	select {
	case err := <-failures:
		if err == nil {
			t.Fatal("query failure swallowed")
		}
	case <-time.After(time.Second):
		t.Fatal("query failure not delivered")
	}
	w.wg.Wait()
	if len(w.active) != 2 {
		t.Fatal("failure erased prior active widgets")
	}
}

func TestWidgetReloadIntervalStartsOnlyAfterSuccessfulReload(t *testing.T) {
	w := newWidgetCoordinator(t.TempDir(), false)
	defer w.close()
	w.active = []string{string(export.KindOverview)}
	w.interval = func() time.Duration { return 15 * time.Minute }
	fail := true
	count := 0
	w.native = &widgetNative{reload: func(kinds []string) error {
		count++
		if fail {
			return errors.New("reload denied")
		}
		return nil
	}}
	now := time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)
	snap := export.NewSnapshot(now, l10n.English, nil, nil)
	if err := w.publish(snap); err == nil {
		t.Fatal("reload failure swallowed")
	}
	if !w.lastReload.IsZero() {
		t.Fatal("failure advanced reload interval")
	}
	fail = false
	if err := w.publish(snap); err != nil {
		t.Fatal(err)
	}
	snap.GeneratedAt = now.Add(time.Minute)
	if err := w.publish(snap); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("reloaded inside interval: %d", count)
	}
	snap.GeneratedAt = now.Add(15 * time.Minute)
	if err := w.publish(snap); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("due reload missing: %d", count)
	}
	if _, err := w.store.Load(); err != nil {
		t.Fatal(err)
	}
}

func TestWidgetCloseCancelsPendingQuery(t *testing.T) {
	w := newWidgetCoordinator(t.TempDir(), false)
	started := make(chan struct{})
	w.native = &widgetNative{configurations: func(ctx context.Context) ([]widgetConfiguration, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	w.onRequirements = func(export.Requirements) { t.Error("closed query applied requirements") }
	w.refresh()
	<-started
	w.close()
	if !w.closed || w.querying {
		t.Fatal("query remained active after close")
	}
}
