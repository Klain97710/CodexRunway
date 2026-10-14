// Package usage reads Codex session logs and turns them into API-equivalent
// cost summaries.
//
// It is the Go port of the Swift usage-cost pipeline:
//
//   - Repository keeps an incremental SQLite index of the session logs
//     (compatible in place with the index file the Swift app wrote, schema 3 /
//     parser 4) and answers windowed queries from it.
//   - Scanner streams the logs once without any index, for one-off scans.
//   - PriceBook / PricingCatalogProvider hold the dated price table and the
//     last-known-good online catalog. Unknown models are never priced.
//   - CacheStore persists the last Summary as JSON.
//   - ApiCostRange, ProfileTokenUsage and the heatmap builders prepare the
//     data the UI draws.
//
// Money is always a decimal.Decimal; token counts are int64 with checked
// arithmetic. Every function that depends on the calendar takes a
// *time.Location, and every function that depends on the clock takes the
// instant as an argument, so results are reproducible in tests.
//
// The package never logs and never touches ~/.codex outside reading the
// session logs; all paths are injected through Config.
package usage
