# Time-zone inputs for the Swift golden tests

These TZif files are test inputs for the timestamp, heatmap, year-to-date and
cost-range snapshots in the parent directory. They preserve the zone rules that
match those Swift-generated expectations, independently of a runner's OS or
`ZONEINFO` environment variable. Missing or invalid files fail the tests.

The snapshot contains all 20 zones used by these comparisons and comes from
macOS `/usr/share/zoneinfo`, version `2026c` (`2026c.1.0`, rearguard format),
captured on 2026-10-13. The underlying IANA time-zone data is public domain.

The first hosted CI run used different Casablanca rules: replaying with IANA
2025b reproduced the same 18 heatmap mismatches and the same one-hour/date
boundary differences. The golden outputs and all assertions remain unchanged;
only their previously implicit time-zone input is now included in the fixture.

When regenerating the Swift goldens, capture the zone inputs from the same
reference environment and update them together after verifying parity. These
files are not embedded in the app and do not override the user's runtime
time-zone database.
