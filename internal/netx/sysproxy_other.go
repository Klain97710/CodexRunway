//go:build !darwin && !windows && !linux

package netx

import "net/url"

// Other systems have no system proxy source; only the environment applies.
func osSystemProxy(*url.URL) (*url.URL, error) { return nil, nil }
