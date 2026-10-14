// Package swiftjson writes JSON that is byte-identical to what the Swift app's
// JSONEncoder produced for the shared on-disk files (status.json,
// widget-snapshot.json, alerts.json, quota-estimate-history.json,
// reaction-visitor.json), and reads the numbers and dates it wrote.
//
// The Swift encoder is configured with .iso8601 dates and sortedKeys, with or
// without prettyPrinted. Go's encoding/json differs from it in several ways
// that matter for a byte comparison: it escapes <, > and & (and U+2028/9) but
// not "/", it prints large and tiny float64 values without an exponent, it
// cannot print a decimal exactly, it writes dates with a fraction, and it never
// pretty-prints the way Foundation does. Marshal reproduces the Foundation
// output. MarshalWithOptions retains the different key order, slash escaping,
// empty-container layout and JSONSerialization number rules of Codex/Grok
// credentials, account indexes and transfer packs.
package swiftjson
