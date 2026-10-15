//go:build race

package app

// Race instrumentation multiplies the framework's layout and text shaping
// cost, so pure input-latency budgets get headroom; non-race runs keep the
// real budget.
const raceLatencyFactor = 5
