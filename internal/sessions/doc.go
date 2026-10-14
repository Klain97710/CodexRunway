// Package sessions reads the Codex session files under ~/.codex for the recent
// sessions list (Scanner) and diagnoses or repairs ~/.codex/session_index.jsonl
// (Repairer). It ports SessionActivity.swift and SessionRepair.swift.
//
// Safety contract, from AGENTS.md: the only file this package ever writes is
// session_index.jsonl, and only after copying it to
// session_index.backup-<unix seconds>.jsonl next to it. Nothing under
// sessions/ or archived_sessions/ is written, moved or deleted, and no other
// global state file is touched.
//
// Differences from the Swift code, all deliberate and all on the side of
// working on Windows and Linux too:
//   - a line ends at "\n" with an optional "\r" before it (Swift's String
//     treats "\r\n" as one character and would not split such a file);
//   - bytes that are not valid UTF-8 are replaced instead of failing the scan;
//   - a project name is the last element of a "/" or "\" separated path;
//   - titles are cut at 80 user-perceived characters with a simplified
//     grapheme rule (see truncateTitle).
package sessions
