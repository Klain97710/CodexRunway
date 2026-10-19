package sessions

import (
	"encoding/json"
	"math"
	"strings"
	"time"
)

func managedMessage(provider string, obj map[string]any) parsedManagedMessage {
	switch provider {
	case ProviderCodex:
		return codexManagedMessage(obj)
	case ProviderClaude:
		return claudeManagedMessage(obj)
	case ProviderGrok:
		return grokManagedMessage(obj)
	}
	return parsedManagedMessage{}
}

func codexManagedMessage(obj map[string]any) parsedManagedMessage {
	payload := managedObject(obj["payload"])
	typ := managedString(payload["type"])
	role := managedString(payload["role"])
	var text string
	source := ""
	if typ == "message" && (role == "user" || role == "assistant") {
		text = managedText(payload["content"])
		source = "codex-response"
	} else if typ == "agent_message" {
		role, text = "assistant", managedString(payload["message"])
		source = "codex-event"
		if text == "" {
			text = managedString(payload["text"])
		}
	}
	return parsedManagedMessage{role: role, text: text, at: managedTime(obj["timestamp"]), key: managedString(payload["id"]), source: source}
}

func claudeManagedMessage(obj map[string]any) parsedManagedMessage {
	role := managedString(obj["type"])
	if role != "user" && role != "assistant" {
		return parsedManagedMessage{}
	}
	message := managedObject(obj["message"])
	text := managedText(message["content"])
	if text == "" {
		text = managedText(obj["content"])
	}
	return parsedManagedMessage{role: role, text: text, at: managedTime(obj["timestamp"]), key: managedString(obj["uuid"]), source: "claude"}
}

func grokManagedMessage(obj map[string]any) parsedManagedMessage {
	params := managedObject(obj["params"])
	update := managedObject(params["update"])
	typ := managedString(update["sessionUpdate"])
	if typ == "turn_completed" {
		return parsedManagedMessage{boundary: true}
	}
	role, text := "", managedText(update["content"])
	chunk := false
	switch typ {
	case "agent_message", "assistant_message":
		role = "assistant"
	case "agent_message_chunk", "assistant_message_chunk":
		role, chunk = "assistant", true
	case "user_message", "human_message":
		role = "user"
	}
	if prompt := managedText(params["prompt"]); prompt != "" {
		role, text = "user", prompt
	} else if prompt := managedText(obj["prompt"]); prompt != "" {
		role, text = "user", prompt
	}
	return parsedManagedMessage{role: role, text: text, at: managedTime(obj["timestamp"]), key: managedString(update["messageId"]), source: "grok", chunk: chunk}
}

func managedObject(value any) map[string]any {
	obj, _ := value.(map[string]any)
	return obj
}

func managedString(value any) string {
	text, _ := value.(string)
	return text
}

func managedText(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case map[string]any:
		if typ := managedString(value["type"]); typ != "" && typ != "text" && typ != "input_text" && typ != "output_text" {
			return ""
		}
		return managedString(value["text"])
	case []any:
		var parts []string
		for _, part := range value {
			if text := managedText(part); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func managedTime(value any) time.Time {
	switch value := value.(type) {
	case string:
		at, _ := time.Parse(time.RFC3339Nano, value)
		return at
	case json.Number:
		seconds, err := value.Float64()
		if err == nil && !math.IsNaN(seconds) && !math.IsInf(seconds, 0) && seconds >= math.MinInt64 && seconds <= math.MaxInt64 {
			whole, fraction := math.Modf(seconds)
			return time.Unix(int64(whole), int64(fraction*1e9))
		}
	}
	return time.Time{}
}
