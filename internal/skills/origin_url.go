package skills

import (
	"net/url"
	"strconv"
	"strings"
)

// NormalizeRepoURL returns an HTTPS repository URL without credentials, query,
// or fragment, and "owner/repo" when it is on github.com.
func NormalizeRepoURL(raw string) (string, string, bool) {
	u, ok := parseOriginURL(raw)
	if !ok {
		return "", "", false
	}
	parts := pathParts(u.Path)
	if len(parts) == 0 {
		return "", "", false
	}
	if u.Host == "github.com" {
		if len(parts) < 2 || invalidRepoParts(parts[:2]) {
			return "", "", false
		}
		parts = parts[:2]
	} else if i := indexPart(parts, "-"); u.Host == "gitlab.com" && i >= 0 {
		parts = parts[:i]
		if invalidRepoParts(parts) {
			return "", "", false
		}
	} else if invalidRepoParts(parts) {
		return "", "", false
	}
	if len(parts) == 0 {
		return "", "", false
	}
	parts[len(parts)-1] = strings.TrimSuffix(parts[len(parts)-1], ".git")
	if parts[len(parts)-1] == "" {
		return "", "", false
	}
	u.Path = "/" + strings.Join(parts, "/")
	repo := u.String()
	github := ""
	if u.Host == "github.com" {
		github = strings.Join(parts[:2], "/")
	}
	return repo, github, true
}

func parseRepoTreeURL(raw string) (repo, ref, subpath string) {
	u, ok := parseOriginURL(raw)
	if !ok {
		return "", "", ""
	}
	parts := pathParts(u.Path)
	var marker int
	switch u.Host {
	case "github.com":
		if len(parts) < 4 || parts[2] != "tree" {
			return "", "", ""
		}
		marker = 2
	case "gitlab.com":
		marker = -1
		for i := 1; i+2 < len(parts); i++ {
			if parts[i] == "-" && parts[i+1] == "tree" {
				marker = i
				break
			}
		}
		if marker < 1 {
			return "", "", ""
		}
	default:
		return "", "", ""
	}
	refIndex := marker + 1
	if u.Host == "gitlab.com" {
		refIndex++
	}
	if refIndex >= len(parts) {
		return "", "", ""
	}
	base := *u
	base.Path = "/" + strings.Join(parts[:marker], "/")
	repo, _, ok = NormalizeRepoURL(base.String())
	if !ok {
		return "", "", ""
	}
	ref = parts[refIndex]
	if ref == "" || ref == "." || ref == ".." {
		return "", "", ""
	}
	if refIndex+1 < len(parts) {
		rawSubpath := strings.Join(parts[refIndex+1:], "/")
		subpath = cleanOriginSkillPath(rawSubpath)
		if subpath == "" && rawSubpath != "" {
			return "", "", ""
		}
	}
	return repo, ref, subpath
}

func normalizeHomepage(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	originalScheme := strings.ToLower(u.Scheme)
	switch originalScheme {
	case "http", "https":
		u.Scheme = "https"
	default:
		return ""
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if host == "www.github.com" {
		host = "github.com"
	}
	if host == "" || !strings.Contains(host, ".") {
		return ""
	}
	u.Host, u.User, u.RawQuery, u.Fragment, u.ForceQuery = host, nil, "", "", false
	if port != "" {
		u.Host += ":" + port
	}
	u.Path, u.RawPath = strings.TrimRight(u.Path, "/"), ""
	return u.String()
}

func parseOriginURL(raw string) (*url.URL, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
		return nil, false
	}
	if converted, ok := scpOriginURL(raw); ok {
		raw = converted
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, false
	}
	originalScheme := strings.ToLower(u.Scheme)
	switch originalScheme {
	case "http", "https", "git", "git+https", "ssh":
		u.Scheme = "https"
	default:
		return nil, false
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if host == "www.github.com" {
		host = "github.com"
	}
	if host == "" || !strings.Contains(host, ".") {
		return nil, false
	}
	u.Host = host
	if port != "" && originalScheme != "ssh" && host != "github.com" {
		u.Host += ":" + port
	}
	u.User, u.RawQuery, u.Fragment, u.ForceQuery = nil, "", "", false
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u, u.Path != ""
}

func scpOriginURL(raw string) (string, bool) {
	if strings.Contains(raw, "://") {
		return "", false
	}
	at := strings.LastIndexByte(raw, '@')
	colon := strings.IndexByte(raw[at+1:], ':')
	if at < 0 || colon < 0 {
		return "", false
	}
	colon += at + 1
	tail := raw[colon+1:]
	if tail == "" || strings.HasPrefix(tail, "/") {
		return "", false
	}
	if _, err := strconv.Atoi(strings.SplitN(tail, "/", 2)[0]); err == nil {
		return "", false
	}
	return "ssh://" + raw[:colon] + "/" + tail, true
}

func pathParts(value string) []string {
	value = strings.Trim(value, "/")
	if value == "" {
		return nil
	}
	return strings.Split(value, "/")
}

func invalidRepoParts(parts []string) bool {
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return true
		}
	}
	return false
}

func indexPart(parts []string, value string) int {
	for i, part := range parts {
		if part == value {
			return i
		}
	}
	return -1
}

func cleanOriginSkillPath(raw string) string {
	raw = strings.Trim(strings.ReplaceAll(raw, "\\", "/"), "/")
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "/")
	cleaned := parts[:0]
	for _, part := range parts {
		if part == ".." {
			return ""
		}
		if part == "" || part == "." {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "/")
}

func originFolder(repo, ref, skillPath string) string {
	if repo == "" || skillPath == "" {
		return ""
	}
	if ref == "" {
		ref = "HEAD"
	}
	if strings.HasPrefix(repo, "https://gitlab.com/") {
		return repo + "/-/tree/" + ref + "/" + skillPath
	}
	if strings.HasPrefix(repo, "https://github.com/") {
		return repo + "/tree/" + ref + "/" + skillPath
	}
	return ""
}
