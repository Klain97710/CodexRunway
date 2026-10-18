package mcp

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/licoy/quosparr/internal/tomlstmt"
)

func loadTOML(source Source, data []byte, fingerprint string) ([]Server, error) {
	root, err := decodeTOML(data)
	if err != nil {
		return nil, err
	}
	raw, exists := root["mcp_servers"]
	if !exists {
		return nil, nil
	}
	servers, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("mcp_servers must be a table")
	}
	out := make([]Server, 0, len(servers))
	for name, rawConfig := range servers {
		config, ok := rawConfig.(map[string]any)
		if !ok {
			return nil, errors.New("MCP server entry must be a table")
		}
		server, err := baseServer(source, name, fingerprint, config)
		if err != nil {
			return nil, err
		}
		out = append(out, server)
	}
	return out, nil
}

func decodeTOML(data []byte) (map[string]any, error) {
	root := map[string]any{}
	if _, err := toml.Decode(string(data), &root); err != nil {
		return nil, errors.New("invalid TOML")
	}
	return root, nil
}

func deleteTOML(data []byte, names map[string]struct{}) ([]byte, error) {
	before, err := decodeTOML(data)
	if err != nil {
		return nil, err
	}
	beforeServers, ok := before["mcp_servers"].(map[string]any)
	if !ok {
		return nil, errors.New("mcp_servers must be a table")
	}
	for name := range names {
		if _, exists := beforeServers[name]; !exists {
			return nil, fmt.Errorf("MCP server %q was not found", name)
		}
		delete(beforeServers, name)
	}
	normalizeEmptyMCPTable(before)
	items, err := tomlstmt.Lex(data)
	if err != nil {
		return nil, errors.New("TOML cannot be edited safely")
	}
	var table []string
	var edits []tomlstmt.Edit
	for _, item := range items {
		if item.Kind == tomlstmt.Header {
			table = item.Path
			if selectedTOMLPath(table, names) {
				edits = append(edits, tomlstmt.Edit{Start: item.Start, End: item.End})
			}
			continue
		}
		if item.Kind != tomlstmt.KeyVal {
			continue
		}
		path := append(append([]string(nil), table...), item.Path...)
		if selectedTOMLPath(path, names) {
			edits = append(edits, tomlstmt.Edit{Start: item.Start, End: item.End})
		}
	}
	if len(edits) == 0 && len(names) > 0 {
		return nil, errors.New("selected MCP declarations were not found")
	}
	out := tomlstmt.ApplyEdits(data, edits)
	remaining, err := decodeTOML(out)
	if err != nil {
		return nil, errors.New("edited TOML is invalid")
	}
	if err := validateRemoved(remaining, names); err != nil {
		return nil, err
	}
	normalizeEmptyMCPTable(remaining)
	if !reflect.DeepEqual(before, remaining) {
		return nil, errors.New("TOML edit changed configuration outside the selection")
	}
	return out, nil
}

func normalizeEmptyMCPTable(root map[string]any) {
	if servers, ok := root["mcp_servers"].(map[string]any); ok && len(servers) == 0 {
		delete(root, "mcp_servers")
	}
}

func selectedTOMLPath(path []string, names map[string]struct{}) bool {
	if len(path) < 2 || path[0] != "mcp_servers" {
		return false
	}
	_, ok := names[path[1]]
	return ok
}

func validateRemoved(root map[string]any, names map[string]struct{}) error {
	raw, ok := root["mcp_servers"]
	if !ok {
		return nil
	}
	servers, ok := raw.(map[string]any)
	if !ok {
		return errors.New("edited mcp_servers is not a table")
	}
	for name := range names {
		if _, exists := servers[name]; exists {
			return fmt.Errorf("MCP server %q was not removed", name)
		}
	}
	return nil
}
