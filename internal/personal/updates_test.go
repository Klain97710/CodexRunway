package personal

import (
	"net/url"
	"testing"
)

func TestPersonalUpdateURLBoundaries(t *testing.T) {
	for _, raw := range []string{
		Homepage + "/releases/latest/download/appcast-arm64.xml",
		Homepage + "/releases/download/personal-v0.2.0/update-darwin-arm64.json",
		"https://api.github.com/repos/" + Repository + "/releases?per_page=100&page=1",
	} {
		u, _ := url.Parse(raw)
		if !UpdateURL(u, false) {
			t.Errorf("valid origin rejected: %s", raw)
		}
	}
	for _, raw := range []string{
		"http://github.com/" + Repository + "/releases/latest/download/a.zip",
		"https://github.com/Licoy/Quosparr/releases/latest/download/a.zip",
		"https://github.com.evil.test/" + Repository + "/releases/latest/download/a.zip",
		"https://u:p@github.com/" + Repository + "/releases/latest/download/a.zip",
		Homepage + "/releases/download/v1.0.3/a.zip", Homepage + "/releases/download/personal-v0.2.0/../a.zip",
		Homepage + "/releases/download/personal-v0.2.0/%2e%2e", Homepage + "/releases/latest/download/a.zip?redirect=evil",
		"https://api.github.com/repos/other/repo/releases", "https://api.github.com/repos/" + Repository + "/releases?next=evil",
		"https://release-assets.githubusercontent.com/github-production-release-asset/other/asset",
	} {
		u, _ := url.Parse(raw)
		if UpdateURL(u, true) {
			t.Errorf("unsafe URL accepted: %s", raw)
		}
	}
	for _, host := range []string{"release-assets.githubusercontent.com", "objects.githubusercontent.com"} {
		u, _ := url.Parse("https://" + host + "/github-production-release-asset/" + RepositoryID + "/asset-uuid?sig=signed")
		if UpdateURL(u, false) || !UpdateURL(u, true) {
			t.Fatal("CDN must require a redirect from our repository")
		}
	}
}
