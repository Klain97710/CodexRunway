package speed

import "time"

const (
	// TailBytes is how much of the end of a log file is read: a session file
	// can be hundreds of megabytes, and the newest part holds the newest calls.
	TailBytes = 16 << 20
	// MaxTTFT is the longest wait still taken as a time to first token: a
	// longer one means the request time was stale (a message injected
	// mid-turn, a retry), not a slow model.
	MaxTTFT = 10 * time.Minute
)
