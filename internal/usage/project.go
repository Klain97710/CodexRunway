package usage

import "strings"

// UnknownProject is the project name for sessions without a working
// directory.
const UnknownProject = "Unknown project"

// ProjectDisplayName returns the folder name of a session's working directory.
// Blank input yields UnknownProject; a path that is only separators is
// returned unchanged. Windows-style paths (drive letter or UNC prefix) are
// split on backslashes as well as slashes.
func ProjectDisplayName(cwd string) string {
	cwd = strings.TrimFunc(cwd, isSwiftWhitespace)
	if cwd == "" {
		return UnknownProject
	}
	name := lastPathComponent(cwd)
	if name == "" {
		return cwd
	}
	return name
}

func looksLikeWindowsPath(p string) bool {
	if strings.HasPrefix(p, `\\`) {
		return true
	}
	return len(p) >= 3 && p[1] == ':' && p[2] == '\\' &&
		(p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z')
}

func lastPathComponent(p string) string {
	if looksLikeWindowsPath(p) {
		p = strings.ReplaceAll(p, `\`, "/")
	}
	trimmed := strings.TrimRight(p, "/")
	if trimmed == "" {
		return "/"
	}
	if i := strings.LastIndexByte(trimmed, '/'); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}
