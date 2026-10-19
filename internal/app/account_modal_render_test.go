package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/store"
)

func TestExportAccountModalReview(t *testing.T) {
	if os.Getenv("RUNWAY_UI_QA") == "" {
		t.Skip("set RUNWAY_UI_QA for account dialog renders")
	}
	dir := filepath.Join("..", "..", ".build", "v1-ui-qa")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		u, tt := accountLayoutFixture(t, store.LanguageSimplifiedChinese, 0, settingsMinWidth, false, accountLayoutRows())
		tt.SetDark(dark)
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, mode := range []string{"paste", modeCodexAPIAdd, modeClaudeAPIAdd, "alias", "note", "switch"} {
			f := accountFormOf(u)
			f.mode, f.alias, f.note, f.switchName = mode, "Personal", "工作账号", "Personal"
			tt.Frame()
			exportReviewPNG(t, tt, filepath.Join(dir, "account-modal-"+mode+"-"+appearance+".png"))
			f.closeEditor()
		}
	}
}
