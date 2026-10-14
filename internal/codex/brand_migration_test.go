package codex

import "testing"

func TestPreviousBrandAccountPackRemainsImportable(t *testing.T) {
	data := []byte(`{"format":"codex-runway-accounts","version":1,"provider":"codex","accounts":[]}`)
	pack, err := DecodeTransferPack(data)
	if err != nil || pack.Format != TransferFormat {
		t.Fatalf("old pack was not normalized: %v", err)
	}
	if _, ok := DecodeTransferPackIfPack(data); !ok {
		t.Fatal("old pack was treated as raw authentication")
	}
	if _, err := EncodeTransferPack(pack); err != nil {
		t.Fatal("imported pack could not be re-exported")
	}
}
