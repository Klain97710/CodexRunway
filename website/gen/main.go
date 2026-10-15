// Command gen renders the static quosparr.com site. Every language gets its
// own prerendered page so search engines index each translation.
//
//	go run ./website/gen -release release.json -out website/dist
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const (
	siteURL = "https://quosparr.com"
	repoURL = "https://github.com/Licoy/Quosparr"
)

func main() {
	root := flag.String("root", ".", "repository root")
	out := flag.String("out", "website/dist", "output directory")
	release := flag.String("release", "", "JSON from `gh release view --json tagName,publishedAt,assets`; omit to derive asset names from mygo.json")
	flag.Parse()
	if err := build(*root, *out, *release); err != nil {
		fmt.Fprintln(os.Stderr, "website:", err)
		os.Exit(1)
	}
}

func build(root, out, releasePath string) error {
	site := filepath.Join(root, "website")
	locales, err := loadLocales(filepath.Join(site, "locales"))
	if err != nil {
		return err
	}
	rel, err := loadRelease(root, releasePath)
	if err != nil {
		return err
	}
	if releasePath == "" {
		fmt.Fprintf(os.Stderr, "website: no -release given, links assume %s exists\n", rel.Tag)
	}
	if err := clearOutput(out); err != nil {
		return err
	}
	return render(root, out, locales, rel)
}

// clearOutput only deletes an empty directory or a previous build, which
// always contains CNAME, so a mistyped -out cannot wipe a source tree.
func clearOutput(out string) error {
	entries, err := os.ReadDir(out)
	if os.IsNotExist(err) || (err == nil && len(entries) == 0) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(out, "CNAME")); err != nil {
		return fmt.Errorf("%s is not empty and is not a previous site build", out)
	}
	return os.RemoveAll(out)
}
