package grok

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/licoy/quosparr/internal/codexapi"
)

// ResetCreditToken is one banked SuperGrok usage-reset card from the official
// ConsumerUiSvc/GetRemainingResets RPC. ValidityStart is nil when the response
// did not say; ValidityEnd is always set for a decoded token.
type ResetCreditToken struct {
	TokenID       string
	ValidityStart *time.Time
	ValidityEnd   *time.Time
}

// IsAvailable reports whether the card can still be used: the official web UI
// keeps a card only while its end lies in the future.
func (t ResetCreditToken) IsAvailable(now time.Time) bool {
	if t.TokenID == "" || t.ValidityEnd == nil {
		return false
	}
	return t.ValidityEnd.After(now)
}

// ResetCreditSnapshot is the decoded GetRemainingResets answer. Tokens keeps
// expired cards; AvailableCount counts only the usable ones at UpdatedAt.
type ResetCreditSnapshot struct {
	AvailableCount int
	Tokens         []ResetCreditToken
	UpdatedAt      time.Time
}

// ErrResetCreditsInvalidFrame, ErrResetCreditsCompressed and
// ErrResetCreditsUnknownStructure are the decoding failures of
// ResetCreditSnapshot; a failed RPC is a *ResetCreditsRPCError.
var (
	ErrResetCreditsInvalidFrame     = errors.New("grok: reset credits: invalid gRPC-Web frame")
	ErrResetCreditsCompressed       = errors.New("grok: reset credits: compressed frames are not supported")
	ErrResetCreditsUnknownStructure = errors.New("grok: reset credits: unknown structure")
)

// ResetCreditsRPCError is a non-zero grpc-status in the response trailer.
type ResetCreditsRPCError struct{ Message string }

func (e *ResetCreditsRPCError) Error() string {
	return "grok: reset credits: rpc failed: " + e.Message
}

// Proto field numbers of prod_mc_billing.ConsumerGetRemainingResetsResp and
// ConsumerResetToken.
const (
	resetFieldTokens        = 10
	resetFieldTokenID       = 10
	resetFieldValidityStart = 20
	resetFieldValidityEnd   = 30
)

// ResetCreditsGRPCWebRequest is the request body of GetRemainingResets: one
// gRPC-Web data frame around message (empty by default).
func ResetCreditsGRPCWebRequest(message []byte) []byte {
	return resetFrame(message, 0)
}

func resetFrame(message []byte, flags byte) []byte {
	out := make([]byte, 5, 5+len(message))
	out[0] = flags
	n := uint32(len(message))
	out[1], out[2], out[3], out[4] = byte(n>>24), byte(n>>16), byte(n>>8), byte(n)
	return append(out, message...)
}

// DecodeResetCredits decodes the official ConsumerGetRemainingResetsResp,
// framed as gRPC-Web, bare protobuf or JSON, as of now.
func DecodeResetCredits(data []byte, now time.Time) (ResetCreditSnapshot, error) {
	payload, err := resetUnwrapEnvelope(data)
	if err != nil {
		return ResetCreditSnapshot{}, err
	}
	switch {
	case len(payload) == 0:
		return resetSnapshot(nil, now), nil
	case resetLooksLikeJSON(payload):
		return resetDecodeJSON(payload, now)
	}
	return resetDecodeProto(payload, now)
}

func resetSnapshot(tokens []ResetCreditToken, now time.Time) ResetCreditSnapshot {
	available := 0
	for _, t := range tokens {
		if t.IsAvailable(now) {
			available++
		}
	}
	if tokens == nil {
		tokens = []ResetCreditToken{}
	}
	return ResetCreditSnapshot{AvailableCount: available, Tokens: tokens, UpdatedAt: now}
}

// CodexSnapshot maps the cards onto the shared reset-credit presentation type.
// A zero now means the snapshot's own UpdatedAt.
func (s ResetCreditSnapshot) CodexSnapshot(now time.Time) codexapi.ResetCreditsSnapshot {
	clock := now
	if clock.IsZero() {
		clock = s.UpdatedAt
	}
	credits := make([]codexapi.ResetCredit, 0, len(s.Tokens))
	for _, t := range s.Tokens {
		credit := codexapi.ResetCredit{ID: t.TokenID, Status: "unavailable"}
		if t.IsAvailable(clock) {
			credit.Status = "available"
		}
		if t.ValidityStart != nil {
			credit.CreatedAt = *t.ValidityStart
		}
		if t.ValidityEnd != nil {
			credit.ExpiresAt = *t.ValidityEnd
			if left := t.ValidityEnd.Sub(clock).Seconds(); left > 0 {
				credit.RemainingSeconds = left
			}
		}
		credits = append(credits, credit)
	}
	return codexapi.ResetCreditsSnapshot{AvailableCount: s.AvailableCount, Credits: credits, UpdatedAt: s.UpdatedAt}
}

// ---- JSON ----

func resetDecodeJSON(data []byte, now time.Time) (ResetCreditSnapshot, error) {
	root, err := parseLooseJSON(data)
	if err != nil || root.kind != jsonObject {
		return ResetCreditSnapshot{}, ErrResetCreditsUnknownStructure
	}
	var tokens []ResetCreditToken
	if list := root.get("tokens"); list != nil && list.kind == jsonArray {
		for _, item := range list.arr {
			if item.kind != jsonObject {
				continue
			}
			if tok, ok := resetJSONToken(item); ok {
				tokens = append(tokens, tok)
			}
		}
	}
	return resetSnapshot(tokens, now), nil
}

func resetJSONToken(obj *jsonNode) (ResetCreditToken, bool) {
	var id string
	for _, key := range []string{"tokenId", "token_id"} {
		if s, ok := obj.get(key).str(); ok {
			if t, ok := billingNonEmpty(s); ok {
				id = t
				break
			}
		}
	}
	if id == "" {
		return ResetCreditToken{}, false
	}
	start := resetJSONDate(resetFirst(obj, "validityStart", "validity_start"))
	end := resetJSONDate(resetFirst(obj, "validityEnd", "validity_end"))
	if end == nil {
		return ResetCreditToken{}, false
	}
	return ResetCreditToken{TokenID: id, ValidityStart: start, ValidityEnd: end}, true
}

// resetFirst is object[a] ?? object[b]: a JSON null counts as present.
func resetFirst(obj *jsonNode, a, b string) *jsonNode {
	if n := obj.get(a); n != nil {
		return n
	}
	return obj.get(b)
}

// resetJSONDate reads a date written as an ISO string, epoch seconds or a
// {seconds, nanos} object.
func resetJSONDate(n *jsonNode) *time.Time {
	if n == nil {
		return nil
	}
	switch n.kind {
	case jsonString:
		if t, ok := BillingParseDate(n.s); ok {
			return &t
		}
	case jsonNumber:
		return resetEpoch(billingNumberDouble(n.s))
	case jsonBool:
		if n.b {
			return resetEpoch(1)
		}
		return resetEpoch(0)
	case jsonObject:
		seconds, ok := resetJSONInt64(n.get("seconds"))
		if !ok {
			return nil
		}
		nanos, _ := resetJSONInt64(n.get("nanos"))
		return resetEpoch(float64(seconds) + float64(nanos)/1e9)
	}
	return nil
}

// resetJSONInt64 reads an integer written as a number (fractions truncate), a
// boolean or a decimal string. Values outside the Int64 range, which the
// Swift app cannot convert, are treated as missing.
func resetJSONInt64(n *jsonNode) (int64, bool) {
	if n == nil {
		return 0, false
	}
	switch n.kind {
	case jsonBool:
		if n.b {
			return 1, true
		}
		return 0, true
	case jsonNumber:
		if v, err := strconv.ParseInt(n.s, 10, 64); err == nil {
			return v, true
		}
		f := billingNumberDouble(n.s)
		if math.IsNaN(f) || f >= 1<<63 || f < -(1<<63) {
			return 0, false
		}
		return int64(f), true
	case jsonString:
		v, err := strconv.ParseInt(n.s, 10, 64)
		return v, err == nil
	}
	return 0, false
}

// resetEpochLimit bounds the instants a time.Time holds comfortably (about a
// billion years); Date(timeIntervalSince1970:) accepts any Double, so a value
// beyond the limit is clamped to it and still counts as far past or future.
const resetEpochLimit = 1 << 55

// resetEpoch is Date(timeIntervalSince1970:). NaN yields nil.
func resetEpoch(seconds float64) *time.Time {
	if math.IsNaN(seconds) {
		return nil
	}
	seconds = math.Max(-resetEpochLimit, math.Min(resetEpochLimit, seconds))
	whole := math.Floor(seconds)
	t := time.Unix(int64(whole), int64(math.Round((seconds-whole)*1e9)))
	return &t
}

// ---- protobuf ----

func resetDecodeProto(data []byte, now time.Time) (ResetCreditSnapshot, error) {
	fields, err := resetProtoFields(data)
	if err != nil {
		return ResetCreditSnapshot{}, err
	}
	var tokens []ResetCreditToken
	for _, v := range fields[resetFieldTokens] {
		if v.bytes == nil {
			continue
		}
		if tok, ok := resetProtoToken(v.bytes); ok {
			tokens = append(tokens, tok)
		}
	}
	return resetSnapshot(tokens, now), nil
}

func resetProtoToken(data []byte) (ResetCreditToken, bool) {
	fields, err := resetProtoFields(data)
	if err != nil {
		return ResetCreditToken{}, false
	}
	id := ""
	for _, v := range fields[resetFieldTokenID] {
		if v.bytes != nil && utf8.Valid(v.bytes) {
			id = string(v.bytes)
			break
		}
	}
	if id == "" {
		return ResetCreditToken{}, false
	}
	start := resetProtoTime(fields[resetFieldValidityStart])
	end := resetProtoTime(fields[resetFieldValidityEnd])
	if end == nil {
		return ResetCreditToken{}, false
	}
	return ResetCreditToken{TokenID: id, ValidityStart: start, ValidityEnd: end}, true
}

// resetProtoTime is the first length-delimited value that parses as a
// google.protobuf.Timestamp.
func resetProtoTime(values []resetProtoValue) *time.Time {
	for _, v := range values {
		if v.bytes == nil {
			continue
		}
		fields, err := resetProtoFields(v.bytes)
		if err != nil {
			continue
		}
		seconds := resetFirstVarint(fields[1])
		nanos := resetFirstVarint(fields[2])
		// Date(timeIntervalSince1970:) of seconds + nanos / 1e9 in doubles; both
		// fields are two's complement Int64 values.
		return resetEpoch(float64(int64(seconds)) + float64(int64(nanos))/1e9)
	}
	return nil
}

func resetFirstVarint(values []resetProtoValue) uint64 {
	for _, v := range values {
		if v.bytes == nil {
			return v.varint
		}
	}
	return 0
}

// resetProtoValue is a varint (bytes == nil) or a length-delimited field.
type resetProtoValue struct {
	varint uint64
	bytes  []byte
}

// resetProtoFields reads the fields of one message. Fixed-width fields are
// skipped, groups and unknown wire types fail.
func resetProtoFields(data []byte) (map[int][]resetProtoValue, error) {
	out := map[int][]resetProtoValue{}
	offset := 0
	for offset < len(data) {
		tag, n, ok := resetReadVarint(data, offset)
		if !ok {
			return nil, ErrResetCreditsUnknownStructure
		}
		offset = n
		field := int(tag >> 3)
		switch tag & 7 {
		case 0:
			v, n, ok := resetReadVarint(data, offset)
			if !ok {
				return nil, ErrResetCreditsUnknownStructure
			}
			offset = n
			out[field] = append(out[field], resetProtoValue{varint: v})
		case 1:
			if offset+8 > len(data) {
				return nil, ErrResetCreditsUnknownStructure
			}
			offset += 8
		case 2:
			length, n, ok := resetReadVarint(data, offset)
			if !ok || length > uint64(len(data)-n) {
				return nil, ErrResetCreditsUnknownStructure
			}
			offset = n
			end := offset + int(length)
			out[field] = append(out[field], resetProtoValue{bytes: append([]byte{}, data[offset:end]...)})
			offset = end
		case 5:
			if offset+4 > len(data) {
				return nil, ErrResetCreditsUnknownStructure
			}
			offset += 4
		default:
			return nil, ErrResetCreditsUnknownStructure
		}
	}
	return out, nil
}

// resetReadVarint reads a base-128 varint; a tenth continuation byte fails.
func resetReadVarint(data []byte, offset int) (uint64, int, bool) {
	var value uint64
	shift := uint(0)
	for i := offset; i < len(data); i++ {
		b := data[i]
		value |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return value, i + 1, true
		}
		shift += 7
		if shift > 63 {
			return 0, 0, false
		}
	}
	return 0, 0, false
}

// ---- gRPC-Web envelope ----

// resetUnwrapEnvelope returns the message of a gRPC-Web response: data
// frames joined, trailer frames checked for a failed status. Anything that
// does not start like a frame is returned untouched.
func resetUnwrapEnvelope(data []byte) ([]byte, error) {
	if len(data) == 0 || resetLooksLikeJSON(data) {
		return data, nil
	}
	if len(data) < 5 || (data[0] != 0 && data[0] != 0x80 && data[0] != 1) {
		return data, nil
	}
	var message []byte
	status := -1
	statusSet := false
	statusMessage := ""
	messageSet := false
	offset := 0
	for offset+5 <= len(data) {
		flags := data[offset]
		length := uint64(data[offset+1])<<24 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<8 | uint64(data[offset+4])
		offset += 5
		if length > uint64(len(data)-offset) {
			return nil, ErrResetCreditsInvalidFrame
		}
		payload := data[offset : offset+int(length)]
		offset += int(length)
		if flags&0x80 != 0 {
			if s, ok := resetTrailerStatus(payload); ok {
				status, statusSet = s, true
			}
			if m, ok := resetTrailerMessage(payload); ok {
				statusMessage, messageSet = m, true
			}
			continue
		}
		if flags&0x01 != 0 {
			return nil, ErrResetCreditsCompressed
		}
		message = append(message, payload...)
	}
	if statusSet && status != 0 {
		if messageSet {
			return nil, &ResetCreditsRPCError{Message: statusMessage}
		}
		return nil, &ResetCreditsRPCError{Message: fmt.Sprintf("grpc-status %d", status)}
	}
	return message, nil
}

// resetTrailerLines yields the (name, value) pairs of a trailer block: the
// name lowercased, both sides trimmed of spaces and tabs.
func resetTrailerLines(payload []byte) [][2]string {
	text := ""
	if utf8.Valid(payload) {
		text = string(payload)
	}
	lines := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '\n', '\r', '\v', '\f', 0x85, 0x2028, 0x2029:
			return true
		}
		return false
	})
	var out [][2]string
	for _, raw := range lines {
		line := billingTrim(raw)
		i := strings.IndexByte(line, ':')
		if i < 0 {
			continue
		}
		name := strings.ToLower(strings.TrimFunc(line[:i], resetIsBlank))
		value := strings.TrimFunc(line[i+1:], resetIsBlank)
		out = append(out, [2]string{name, value})
	}
	return out
}

// resetIsBlank matches CharacterSet.whitespaces (no newlines).
func resetIsBlank(r rune) bool { return r == '\t' || (isBillingSpace(r) && !resetIsNewline(r)) }

func resetIsNewline(r rune) bool {
	switch r {
	case '\n', '\v', '\f', '\r', 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// resetTrailerStatus is the last grpc-status line that parses as an integer.
func resetTrailerStatus(payload []byte) (int, bool) {
	found, status := false, 0
	for _, kv := range resetTrailerLines(payload) {
		if kv[0] != "grpc-status" {
			continue
		}
		if v, err := strconv.Atoi(kv[1]); err == nil {
			found, status = true, v
		} else {
			found = false // Int(value) is nil and replaces an earlier status
		}
	}
	return status, found
}

// resetTrailerMessage is the last grpc-message line, percent-decoded.
func resetTrailerMessage(payload []byte) (string, bool) {
	found, msg := false, ""
	for _, kv := range resetTrailerLines(payload) {
		if kv[0] == "grpc-message" {
			found, msg = true, resetPercentDecode(kv[1])
		}
	}
	return msg, found
}

// resetPercentDecode is String.removingPercentEncoding with the original text
// as fallback: a malformed escape or a result that is not UTF-8 keeps it.
func resetPercentDecode(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			out = append(out, s[i])
			continue
		}
		if i+2 >= len(s) {
			return s
		}
		hi, ok1 := resetHexVal(s[i+1])
		lo, ok2 := resetHexVal(s[i+2])
		if !ok1 || !ok2 {
			return s
		}
		out = append(out, hi<<4|lo)
		i += 2
	}
	if !utf8.Valid(out) {
		return s
	}
	return string(out)
}

func resetHexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func resetLooksLikeJSON(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		}
		return b == '{' || b == '['
	}
	return false
}
