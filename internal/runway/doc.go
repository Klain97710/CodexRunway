// Package runway is the Codex-side application coordinator. A Service owns
// the background refresh work (quota, reset credits, cost, heatmap,
// estimate, sessions, alerts, exports) and publishes immutable Snapshot
// values; it never draws. The UI reads snapshots on its own thread, so it
// never parses auth JSON or scans files.
//
// Every asynchronous result is tagged with the account it was fetched for and
// a generation. After an account switch, an authentication failure or a
// cancel, results of the old generation are dropped instead of applied.
//
// Grok and the "rate limit reset today" feature are separate packages; the
// Service only exposes typed seams (GrokProvider, ResetTodayProvider) whose
// output is carried verbatim in the Snapshot.
package runway
