package codex

import "testing"

func TestMaskAccountText(t *testing.T) {
	tests := []struct{ in, want string }{
		{"example@qq.com", "e***@***.com"},
		{"maxwmm1999@outlook.com", "m***@***.com"},
		{"Example@QQ.COM", "E***@***.COM"},
		{"  example@qq.com  ", "e***@***.com"},
		{"user@mail.qq.com", "u***@***.com"},
		{"a@qq.com", "a@***.com"},
		{"user@localhost", "u***@***"},
		{"alice", "a***"},
		{"张三", "张***"},
		{"a", "a"},
		{"", ""},
		{"   ", "   "},
		{"example@qq.com: missing", "e***@***.com: missing"},
		{"éx", "é***"},             // combining mark stays with its base
		{"🇨🇳🇺🇸", "🇨🇳***"},            // a flag is one character
		{"👨‍👩‍👧 family", "👨‍👩‍👧***"}, // ZWJ sequence is one character
	}
	for _, tc := range tests {
		if got := MaskAccountText(tc.in); got != tc.want {
			t.Errorf("MaskAccountText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMaskEmailsIn(t *testing.T) {
	tests := []struct{ in, want string }{
		{"see example@qq.com.", "see e***@***.com."},
		{"no address here", "no address here"},
		{"mail me (a@b.co), or <c@d.io>;", "mail me (a@***.co), or <c@***.io>;"},
		{"@x.com", "@x.com"},
		{"a@", "a@"},
		{"x@y@z.com", "x@***@z.com"},
	}
	for _, tc := range tests {
		if got := MaskEmailsIn(tc.in); got != tc.want {
			t.Errorf("MaskEmailsIn(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMaskIsStable(t *testing.T) {
	masked := MaskAccountText("example@qq.com")
	if again := MaskAccountText(masked); again != masked {
		t.Fatalf("second pass changed %q to %q", masked, again)
	}
}
