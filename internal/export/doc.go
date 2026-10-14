// Package export writes the files other programs read: the status JSON for
// scripts (status.json), the widget snapshot for the WidgetKit extension
// (widget-snapshot.json) and the widget's deep-link and layout contract.
//
// Everything is encoded by internal/export/swiftjson so the bytes equal what
// the Swift JSONEncoder produced; the existing Swift widget extension keeps
// decoding them unchanged.
//
// The quota, cost and reset-today models are not ported yet, so this package
// declares the minimal shapes it writes: QuotaInput, CostSummary (a mirror of
// ApiEquivalentSummary) and the ResetTodaySource interface that supplies the
// verdict logic for the widget timeline. Callers convert into these.
package export
