// Package resettoday is the Go port of Quosparr's "did Codex reset today"
// feature: the hosted status feed (RateLimitResetToday*.swift), its strict
// decoder, the verdict, next-scheduled-reset rules and public status refresh.
//
// Time rules. Feed timestamps are absolute RFC 3339 instants. "Today" is the
// viewer's local Gregorian day, passed to every method as a *time.Location
// (nil means time.Local); it is never the UTC date prefix of a feed string.
// Date-precision schedules are published in the source's calendar day
// (America/Los_Angeles), which is why this package embeds the time zone
// database. Midnights that do not exist (DST gaps) resolve to the first valid
// instant of the day, as Foundation does.
//
// Text. Verdict copy goes through internal/l10n. The few strings that need
// internal/runway's duration and schedule formatting take a Formatter, so this
// package does not depend on the coordinator that consumes it.
//
// Privacy. Public status requests never send or store cookies. There is no
// reaction endpoint or visitor file access. Nothing here reads credentials.
package resettoday
