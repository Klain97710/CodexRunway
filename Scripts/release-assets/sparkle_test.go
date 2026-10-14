package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

func TestSparkleAppcastSignsArchiveForSUPublicEDKey(t *testing.T) {
	dir := t.TempDir()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "mygo.json")
	config := `{"version":"1.2.3","macos":{"minimumSystemVersion":"12.0","infoPlist":{"CFBundleVersion":"123","SUPublicEDKey":"` + base64.StdEncoding.EncodeToString(pub) + `"}}}`
	if err := os.WriteFile(cfg, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "Quosparr-macos-arm64.zip")
	if err := os.WriteFile(archive, []byte("archive fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	feed := filepath.Join(dir, "quosparr-appcast-arm64.xml")
	url := "https://github.com/Licoy/Quosparr/releases/download/v1.2.3/Quosparr-macos-arm64.zip"
	args := []string{"sparkle-appcast", "-config", cfg, "-file", archive, "-url", url, "-out", feed}

	t.Setenv("SPARKLE_PRIVATE_KEY", "")
	if err := run(args); err == nil {
		t.Fatal("appcast written without a signing key")
	}
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	t.Setenv("SPARKLE_PRIVATE_KEY", base64.StdEncoding.EncodeToString(other.Seed()))
	if err := run(args); err == nil {
		t.Fatal("appcast signed with a key that does not match SUPublicEDKey")
	}
	t.Setenv("SPARKLE_PRIVATE_KEY", base64.StdEncoding.EncodeToString(key.Seed()))
	bad := append([]string(nil), args...)
	bad[6] = "https://github.com/Licoy/Quosparr/releases/download/v1.2.3/other.zip"
	if err := run(bad); err == nil {
		t.Fatal("appcast accepted a URL naming another archive")
	}
	if err := run(args); err != nil {
		t.Fatal(err)
	}

	var rss struct {
		Items []struct {
			Version   string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle version"`
			Short     string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle shortVersionString"`
			Minimum   string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle minimumSystemVersion"`
			Enclosure struct {
				URL       string `xml:"url,attr"`
				Length    int    `xml:"length,attr"`
				Signature string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle edSignature,attr"`
			} `xml:"enclosure"`
		} `xml:"channel>item"`
	}
	data, err := os.ReadFile(feed)
	if err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal(data, &rss); err != nil {
		t.Fatal(err)
	}
	if len(rss.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(rss.Items))
	}
	item := rss.Items[0]
	if item.Version != "123" || item.Short != "1.2.3" || item.Minimum != "12.0" || item.Enclosure.URL != url || item.Enclosure.Length != len("archive fixture") {
		t.Fatalf("unexpected item: %+v", item)
	}
	if err := run([]string{"verify-sparkle", "-config", cfg, "-file", archive, "-signature", item.Enclosure.Signature}); err != nil {
		t.Fatal(err)
	}
}
