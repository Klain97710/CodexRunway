package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type jsonDocument map[string]json.RawMessage

func loadJSON(source Source, data []byte, fingerprint string) ([]Server, error) {
	_, servers, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}
	out := make([]Server, 0, len(servers))
	for name, raw := range servers {
		config, err := decodeJSONObject(raw)
		if err != nil {
			return nil, errors.New("MCP server entry must be an object")
		}
		server, err := baseServer(source, name, fingerprint, config)
		if err != nil {
			return nil, err
		}
		out = append(out, server)
	}
	return out, nil
}

func decodeJSON(data []byte) (jsonDocument, map[string]json.RawMessage, error) {
	if err := validateUniqueJSON(data); err != nil {
		return nil, nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var document jsonDocument
	if err := decoder.Decode(&document); err != nil || document == nil {
		return nil, nil, errors.New("invalid JSON object")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, nil, errors.New("JSON has trailing content")
	}
	raw, exists := document["mcpServers"]
	if !exists {
		return document, nil, nil
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &servers); err != nil || servers == nil {
		return nil, nil, errors.New("mcpServers must be an object")
	}
	return document, servers, nil
}

func validateUniqueJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := validateJSONValue(decoder); err != nil {
		return errors.New("invalid JSON object")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("JSON has trailing content")
	}
	return nil
}

func validateJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, duplicate := seen[key]; duplicate {
				return errors.New("duplicate JSON object key")
			}
			seen[key] = struct{}{}
			if err := validateJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := validateJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return errors.New("unexpected JSON delimiter")
	}
}

func decodeJSONObject(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, errors.New("not an object")
	}
	return value, nil
}

func deleteJSON(data []byte, names map[string]struct{}) ([]byte, error) {
	document, servers, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}
	for name := range names {
		if _, ok := servers[name]; !ok {
			return nil, fmt.Errorf("MCP server %q was not found", name)
		}
		delete(servers, name)
	}
	replacement, err := json.Marshal(servers)
	if err != nil {
		return nil, errors.New("cannot encode mcpServers")
	}
	document["mcpServers"] = replacement
	out, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, errors.New("cannot encode configuration")
	}
	out = append(out, '\n')
	_, remaining, err := decodeJSON(out)
	if err != nil {
		return nil, errors.New("edited JSON is invalid")
	}
	for name := range names {
		if _, ok := remaining[name]; ok {
			return nil, fmt.Errorf("MCP server %q was not removed", name)
		}
	}
	return out, nil
}
