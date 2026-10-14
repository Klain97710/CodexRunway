package grok

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Chat-proxy identity headers. The values match the local Grok CLI.
const (
	fallbackClientVersion = "0.2.93"
	tokenAuthHeader       = "X-XAI-Token-Auth"
	tokenAuthValue        = "xai-grok-cli"
	clientVersionHeader   = "x-grok-client-version"
	clientIDHeader        = "x-grok-client-identifier"
	clientIDValue         = "grok-shell"
	clientModeHeader      = "x-grok-client-mode"
	clientModeValue       = "headless"
	userAgentPrefix       = "xai-grok-workspace/"
)

// semanticVersion is the x.y / x.y.z token the CLI prints, with an optional
// pre-release suffix and no spaces.
var semanticVersion = regexp.MustCompile(`\b(\d+\.\d+(?:\.\d+)?(?:-[0-9A-Za-z.-]+)?)\b`)

// parseClientVersion reads `grok --version` output. Empty when nothing in it
// looks like a version.
func parseClientVersion(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	tokens := strings.Fields(trimmed)
	for i, tok := range tokens {
		if strings.EqualFold(tok, "grok") && i+1 < len(tokens) {
			if v := firstSemanticVersion(tokens[i+1]); v != "" {
				return v
			}
		}
	}
	return firstSemanticVersion(trimmed)
}

func firstSemanticVersion(text string) string {
	m := semanticVersion.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// normalizedClientVersion prefers a parsed version, then the trimmed text,
// then the fallback used when the CLI is missing.
func normalizedClientVersion(raw string) string {
	if v := parseClientVersion(raw); v != "" {
		return v
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallbackClientVersion
	}
	return trimmed
}

func userAgent(version string) string {
	return userAgentPrefix + normalizedClientVersion(version)
}

var (
	versionOnce sync.Once
	versionText string
)

// defaultClientVersion runs `grok --version` once. A missing binary or an
// unreadable answer uses the fallback. Tests inject their own version and
// do not call this.
func defaultClientVersion() string {
	versionOnce.Do(func() { versionText = readGrokVersion() })
	return versionText
}

func readGrokVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "grok", "--no-auto-update", "--version").Output()
	if err != nil {
		return fallbackClientVersion
	}
	if v := parseClientVersion(string(out)); v != "" {
		return v
	}
	return fallbackClientVersion
}
