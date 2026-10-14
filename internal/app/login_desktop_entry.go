package app

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
)

func previousExecutable(path string) bool {
	// Entry identity and exact login-only arguments are checked by each caller.
	name := strings.ToLower(filepath.Base(strings.ReplaceAll(path, `\`, "/")))
	return name == "codexrunway" || name == "codex-runway" || name == "quosparr" || name == "codexrunway.exe" || name == "codex-runway.exe" || name == "quosparr.exe" ||
		(strings.HasSuffix(name, ".appimage") && (strings.HasPrefix(name, "codexrunway-") || strings.HasPrefix(name, "codex-runway-") || strings.HasPrefix(name, "quosparr-")))
}

// Decode precisely the .desktop format written by mygo, without executing Exec.
func previousDesktopEntry(data []byte) (bool, error) {
	fields := map[string]string{}
	inside := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inside = line == "[Desktop Entry]"
			continue
		}
		if !inside || strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if _, exists := fields[key]; exists {
			return false, errors.New("ambiguous legacy desktop entry; retained")
		}
		fields[key] = value
	}
	command := fields["Exec"]
	if fields["Type"] != "Application" || !strings.HasPrefix(command, `"`) || !strings.HasSuffix(command, `" --mygo-login`) {
		return false, errors.New("unrecognised legacy desktop entry; retained")
	}
	executable := strings.TrimSuffix(command[1:], `" --mygo-login`)
	executable = strings.NewReplacer(`\\\\`, `\`, `\\"`, `"`, `\\$`, `$`, `\\`+"`", "`", "%%", "%").Replace(executable)
	// A .desktop entry always contains a Unix path, including when parsed on Windows.
	if !path.IsAbs(executable) || !previousExecutable(executable) {
		return false, errors.New("legacy desktop target cannot be identified; retained")
	}
	return fields["Hidden"] != "true" && fields["X-GNOME-Autostart-enabled"] != "false", nil
}
