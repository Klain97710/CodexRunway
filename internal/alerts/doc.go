// Package alerts decides which quota, reset-credit and rate-limit-reset
// notifications are new. It ports RunwayAlerts.swift: the alert IDs, the
// thresholds, the dedupe rules and the alerts.json state file are the Swift
// ones, so a migrated installation does not re-announce what it already showed.
//
// Delivering an OS notification is not done here; callers show the alerts that
// Store.Unseen returns.
//
// The quota, reset-credit and reset-today inputs are small local structs
// (QuotaInput, ResetCredit, ResetTodayState) because the full models live in
// other packages. The caller fills in what the Swift snapshot computes itself:
// whether the reset-today verdict is unavailable, and the next scheduled reset.
package alerts
