package claude

import (
	"bytes"
	"errors"
	"os"
	"strings"
)

// ErrPastedCredential means the text has no claudeAiOauth object.
// The error does not echo the pasted text.
var ErrPastedCredential = errors.New("claude: pasted text has no claudeAiOauth credential")

// ParsePasted accepts one JSON document that contains claudeAiOauth.
func ParsePasted(text string) ([]byte, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrPastedCredential
	}
	if _, err := oauthObject([]byte(text)); err != nil {
		return nil, ErrPastedCredential
	}
	return []byte(text), nil
}

// ReadCredentialFile reads a credentials JSON file. The bytes are not logged.
func ReadCredentialFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if _, err := oauthObject(bytes.TrimSpace(data)); err != nil {
		return nil, ErrPastedCredential
	}
	return data, nil
}
