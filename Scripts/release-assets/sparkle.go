package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// sparkleConfig is the part of mygo.json the Sparkle feed of the Swift
// releases needs; those apps share the bundle identifier and SUPublicEDKey.
type sparkleConfig struct {
	Version string
	MacOS   struct {
		MinimumSystemVersion string
		InfoPlist            struct{ SUPublicEDKey, CFBundleVersion string }
	}
}

// sparkleKey reads SPARKLE_PRIVATE_KEY, the base64 32-byte seed that
// Sparkle's generate_keys exports, and checks it against SUPublicEDKey.
func sparkleKey(publicKey string) (ed25519.PrivateKey, error) {
	pub, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKey))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("invalid Sparkle public key")
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("SPARKLE_PRIVATE_KEY")))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("missing or invalid Sparkle signing key (expected the base64 seed from generate_keys -x)")
	}
	key := ed25519.NewKeyFromSeed(seed)
	if !ed25519.PublicKey(pub).Equal(key.Public()) {
		return nil, errors.New("Sparkle signing key does not match SUPublicEDKey")
	}
	return key, nil
}

// sparkleAppcast writes a one-item Sparkle feed for an archive, signed as
// Sparkle's generate_appcast does: Ed25519 over the archive bytes.
func sparkleAppcast(args []string) error {
	flags := flag.NewFlagSet("sparkle-appcast", flag.ContinueOnError)
	cfgPath := flags.String("config", "mygo.json", "configuration")
	file := flags.String("file", "", "archive")
	download := flags.String("url", "", "archive download URL")
	out := flags.String("out", "", "appcast output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	var cfg sparkleConfig
	if err := readJSON(*cfgPath, &cfg); err != nil {
		return err
	}
	if cfg.Version == "" || cfg.MacOS.InfoPlist.CFBundleVersion == "" || cfg.MacOS.MinimumSystemVersion == "" {
		return errors.New("configuration lacks version, CFBundleVersion or minimumSystemVersion")
	}
	u, err := url.Parse(*download)
	if err != nil || u.Scheme != "https" || u.Host == "" || path.Base(u.Path) != filepath.Base(*file) {
		return errors.New("download URL must be https and name the archive")
	}
	key, err := sparkleKey(cfg.MacOS.InfoPlist.SUPublicEDKey)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("empty archive")
	}
	feed := appcastXML(cfg, *download, len(data), base64.StdEncoding.EncodeToString(ed25519.Sign(key, data)), time.Now().UTC())
	return os.WriteFile(*out, feed, 0644)
}

func appcastXML(cfg sparkleConfig, download string, length int, signature string, date time.Time) []byte {
	var b bytes.Buffer
	text := func(s string) string {
		var e bytes.Buffer
		xml.EscapeText(&e, []byte(s))
		return e.String()
	}
	b.WriteString(xml.Header)
	b.WriteString(`<rss xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle" version="2.0">` + "\n")
	b.WriteString("  <channel>\n    <title>Quosparr</title>\n    <item>\n")
	fmt.Fprintf(&b, "      <title>%s</title>\n", text(cfg.Version))
	fmt.Fprintf(&b, "      <pubDate>%s</pubDate>\n", date.Format(time.RFC1123Z))
	fmt.Fprintf(&b, "      <sparkle:version>%s</sparkle:version>\n", text(cfg.MacOS.InfoPlist.CFBundleVersion))
	fmt.Fprintf(&b, "      <sparkle:shortVersionString>%s</sparkle:shortVersionString>\n", text(cfg.Version))
	fmt.Fprintf(&b, "      <sparkle:minimumSystemVersion>%s</sparkle:minimumSystemVersion>\n", text(cfg.MacOS.MinimumSystemVersion))
	fmt.Fprintf(&b, "      <enclosure url=\"%s\" length=\"%d\" type=\"application/octet-stream\" sparkle:edSignature=\"%s\"/>\n",
		text(download), length, text(signature))
	b.WriteString("    </item>\n  </channel>\n</rss>\n")
	return b.Bytes()
}
