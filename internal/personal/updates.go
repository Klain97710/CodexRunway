package personal

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

const (
	Feedback     = Homepage + "/issues/new"
	Releases     = Homepage + "/releases/latest"
	TagPrefix    = "personal-v"
	RepositoryID = "1401041367"
)

var releaseTag = regexp.MustCompile(`^personal-v[0-9]+\.[0-9]+\.[0-9]+$`)
var assetName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// UpdateURL allows only this repository and its release-asset CDN namespace.
// CDN URLs are accepted only as redirects, never as manifest-supplied origins.
func UpdateURL(u *url.URL, redirected bool) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.Fragment != "" ||
		u.RawPath != "" || strings.ContainsAny(u.Path, "\\%\r\n\x00") || path.Clean(u.Path) != u.Path {
		return false
	}
	switch u.Host {
	case "api.github.com":
		if u.Path != "/repos/"+Repository+"/releases" {
			return false
		}
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return false
		}
		for key, values := range q {
			if (key != "per_page" && key != "page") || len(values) != 1 || !regexpDigits(values[0]) {
				return false
			}
		}
		return true
	case "github.com":
		if u.RawQuery != "" {
			return false
		}
		rest, ok := strings.CutPrefix(u.Path, "/"+Repository+"/releases/")
		if !ok {
			return false
		}
		parts := strings.Split(rest, "/")
		if len(parts) != 3 || !assetName.MatchString(parts[2]) {
			return false
		}
		return (parts[0] == "download" && releaseTag.MatchString(parts[1])) || (parts[0] == "latest" && parts[1] == "download")
	case "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		return redirected && len(parts) == 3 && parts[0] == "github-production-release-asset" && parts[1] == RepositoryID && assetName.MatchString(parts[2])
	}
	return false
}

func regexpDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
