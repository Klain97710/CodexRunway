// Package codexapi talks to the ChatGPT/Codex backend: quota and reset credit
// requests, the OAuth refresh and login (PKCE) exchanges, and the models they
// return. Every request takes a context and an *http.Client built by
// internal/netx; this package never reads or writes a credential file, so the
// auth and account storage layer owns persistence (see TokenPersister).
package codexapi
