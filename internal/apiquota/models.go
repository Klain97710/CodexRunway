package apiquota

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	modelsTimeout      = 15 * time.Second
	maxModelsBodyBytes = 4 << 20
	anthropicVersion   = "2023-06-01"
)

// ModelStyle selects how FetchModels authenticates.
type ModelStyle int

const (
	// ModelStyleOpenAI sends Authorization: Bearer <key>.
	ModelStyleOpenAI ModelStyle = iota
	// ModelStyleAnthropic sends x-api-key and anthropic-version, and retries
	// once with a Bearer token when the site rejects the key header.
	ModelStyleAnthropic
)

// Errors returned by FetchModels. They never contain the key or a body.
var (
	ErrUnauthorized    = errors.New("unauthorized")
	ErrModelsNotFound  = errors.New("models endpoint not found")
	ErrInvalidResponse = errors.New("invalid models response")
	ErrTimeout         = errors.New(msgTimedOut)
	errNoClient        = errors.New("no HTTP client")
)

// HTTPStatusError is an unexpected HTTP status from the models endpoint.
type HTTPStatusError struct{ Code int }

func (e *HTTPStatusError) Error() string { return "HTTP " + strconv.Itoa(e.Code) }

// FetchModels lists the model ids of the site (GET on the models URL),
// de-duplicated and sorted. Failures are returned as the sentinel errors above,
// *HTTPStatusError, context.Canceled, or a sanitized transport error.
func FetchModels(ctx context.Context, client Doer, baseURL, apiKey string, style ModelStyle) ([]string, error) {
	if client == nil {
		return nil, errNoClient
	}
	urls, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(apiKey)
	ctx, cancel := context.WithTimeout(ctx, modelsTimeout)
	defer cancel()

	status, body, err := fetchModelsBody(ctx, client, urls.ModelsURL, key, style)
	if err != nil {
		return nil, transportError(err, key)
	}
	switch {
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return nil, ErrUnauthorized
	case status == http.StatusNotFound:
		return nil, ErrModelsNotFound
	case status < 200 || status >= 300:
		return nil, &HTTPStatusError{Code: status}
	}
	return parseModelIDs(body)
}

func fetchModelsBody(ctx context.Context, client Doer, modelsURL, key string, style ModelStyle) (int, []byte, error) {
	status, body, err := getModels(ctx, client, modelsURL, key, style, false)
	if err == nil && style == ModelStyleAnthropic && key != "" &&
		(status == http.StatusUnauthorized || status == http.StatusForbidden) {
		return getModels(ctx, client, modelsURL, key, style, true)
	}
	return status, body, err
}

func getModels(ctx context.Context, client Doer, modelsURL, key string, style ModelStyle, bearer bool) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return 0, nil, ErrInvalidBaseURL
	}
	req.Header.Set("Accept", "application/json")
	switch {
	case key == "":
	case style == ModelStyleAnthropic && !bearer:
		req.Header.Set("x-api-key", key)
	default:
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if style == ModelStyleAnthropic {
		req.Header.Set("anthropic-version", anthropicVersion)
	}
	return doRequest(client, req, maxModelsBodyBytes)
}

// transportError maps a transport failure to a sanitized error.
func transportError(err error, key string) error {
	switch {
	case errors.Is(err, ErrInvalidBaseURL):
		return err
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded), isTimeout(err):
		return ErrTimeout
	}
	return errors.New(requestMessage(err, key))
}

// parseModelIDs reads {"data":[{"id":...}]}, the shape shared by the OpenAI
// and Anthropic model lists. A missing or null data list is an empty list.
func parseModelIDs(body []byte) ([]string, error) {
	value, ok := decodeJSON(string(body))
	if !ok {
		return nil, ErrInvalidResponse
	}
	o, ok := asObject(value)
	if !ok {
		return nil, ErrInvalidResponse
	}
	ids := []string{}
	switch data := o["data"].(type) {
	case nil:
	case []any:
		for _, item := range data {
			m, _ := asObject(item)
			if id, _ := fieldString(m, "id"); strings.TrimSpace(id) != "" {
				ids = append(ids, strings.TrimSpace(id))
			}
		}
	default:
		return nil, ErrInvalidResponse
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}
