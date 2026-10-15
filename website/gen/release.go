package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Asset struct {
	Name string
	URL  string
	Size int64
}

// HumanSize is empty when the size is unknown (no release JSON).
func (a Asset) HumanSize() string {
	if a.Size <= 0 {
		return ""
	}
	return fmt.Sprintf("%.1f MB", float64(a.Size)/(1<<20))
}

type Downloads struct {
	MacArm64DMG, MacArm64ZIP, MacX64DMG, MacX64ZIP Asset
	WinX64, WinArm64                               Asset
	WinX64Portable, WinArm64Portable               Asset
	LinuxX64Deb, LinuxArm64Deb                     Asset
	LinuxX64Tar, LinuxArm64Tar                     Asset
	InstallScript, Checksums                       Asset
}

type Release struct {
	Tag     string
	Version string
	Date    string // YYYY-MM-DD, empty without release JSON
	Page    string
	DL      Downloads
}

type ghRelease struct {
	TagName     string
	PublishedAt string
	Assets      []struct {
		Name string
		Size int64
	}
}

type assetRule struct {
	dst      *Asset
	pattern  string
	fallback string // %s is the version
}

func (d *Downloads) rules() []assetRule {
	return []assetRule{
		{&d.MacArm64DMG, `^Quosparr-macos-arm64\.dmg$`, "Quosparr-macos-arm64.dmg"},
		{&d.MacArm64ZIP, `^Quosparr-macos-arm64\.zip$`, "Quosparr-macos-arm64.zip"},
		{&d.MacX64DMG, `^Quosparr-macos-x86_64\.dmg$`, "Quosparr-macos-x86_64.dmg"},
		{&d.MacX64ZIP, `^Quosparr-macos-x86_64\.zip$`, "Quosparr-macos-x86_64.zip"},
		{&d.WinX64, `^Quosparr[. ]Setup[. ].+[. ]amd64\.exe$`, "Quosparr.Setup.%s.amd64.exe"},
		{&d.WinArm64, `^Quosparr[. ]Setup[. ].+[. ]arm64\.exe$`, "Quosparr.Setup.%s.arm64.exe"},
		{&d.WinX64Portable, `^quosparr-.+-windows-amd64\.tar\.gz$`, "quosparr-%s-windows-amd64.tar.gz"},
		{&d.WinArm64Portable, `^quosparr-.+-windows-arm64\.tar\.gz$`, "quosparr-%s-windows-arm64.tar.gz"},
		{&d.LinuxX64Deb, `^quosparr_.+_amd64\.deb$`, "quosparr_%s_amd64.deb"},
		{&d.LinuxArm64Deb, `^quosparr_.+_arm64\.deb$`, "quosparr_%s_arm64.deb"},
		{&d.LinuxX64Tar, `^quosparr-.+-linux-amd64\.tar\.gz$`, "quosparr-%s-linux-amd64.tar.gz"},
		{&d.LinuxArm64Tar, `^quosparr-.+-linux-arm64\.tar\.gz$`, "quosparr-%s-linux-arm64.tar.gz"},
		{&d.InstallScript, `^install\.sh$`, "install.sh"},
		{&d.Checksums, `^SHA256SUMS$`, "SHA256SUMS"},
	}
}

func loadRelease(root, path string) (Release, error) {
	if path == "" {
		return releaseFromManifest(root)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Release{}, err
	}
	var gh ghRelease
	if err := json.Unmarshal(data, &gh); err != nil {
		return Release{}, fmt.Errorf("%s: %w", path, err)
	}
	return releaseFromGitHub(gh)
}

// releaseFromGitHub fails when an expected asset is missing so a deploy never
// publishes a dead download link.
func releaseFromGitHub(gh ghRelease) (Release, error) {
	if !strings.HasPrefix(gh.TagName, "v") {
		return Release{}, fmt.Errorf("unexpected release tag %q", gh.TagName)
	}
	rel := newRelease(gh.TagName)
	if len(gh.PublishedAt) >= 10 {
		rel.Date = gh.PublishedAt[:10]
	}
	for _, rule := range rel.DL.rules() {
		re := regexp.MustCompile(rule.pattern)
		var found []Asset
		for _, a := range gh.Assets {
			if re.MatchString(a.Name) {
				found = append(found, Asset{Name: a.Name, Size: a.Size})
			}
		}
		if len(found) != 1 {
			return Release{}, fmt.Errorf("%s: expected one asset matching %s, found %d", gh.TagName, rule.pattern, len(found))
		}
		*rule.dst = rel.asset(found[0])
	}
	return rel, nil
}

func releaseFromManifest(root string) (Release, error) {
	data, err := os.ReadFile(filepath.Join(root, "mygo.json"))
	if err != nil {
		return Release{}, err
	}
	var manifest struct{ Version string }
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version == "" {
		return Release{}, fmt.Errorf("mygo.json: missing version")
	}
	rel := newRelease("v" + manifest.Version)
	for _, rule := range rel.DL.rules() {
		name := rule.fallback
		if strings.Contains(name, "%s") {
			name = fmt.Sprintf(name, rel.Version)
		}
		*rule.dst = rel.asset(Asset{Name: name})
	}
	return rel, nil
}

func newRelease(tag string) Release {
	return Release{Tag: tag, Version: strings.TrimPrefix(tag, "v"), Page: repoURL + "/releases/tag/" + tag}
}

func (r Release) asset(a Asset) Asset {
	a.URL = repoURL + "/releases/download/" + r.Tag + "/" + a.Name
	return a
}
