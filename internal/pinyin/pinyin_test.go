package pinyin

import "testing"

// Known words, verified the way the old build's table was: the initials are
// what every other launcher types.
func TestInitialsOfKnownWords(t *testing.T) {
	cases := map[string]string{
		"网易云音乐":              "wyyyy",
		"企业微信":               "qywx",
		"浏览器":                "llq",
		"剪贴板":                "jtb",
		"计算器":                "jsq",
		"重启":                 "cq",
		"关机":                 "gj",
		"设置":                 "sz",
		"终端":                 "zd",
		"退出":                 "tc",
		"Visual Studio Code": "visualstudiocode",
		"微信":                 "wx",
		"文件管理器":              "wjglq",
		"腾讯视频":               "txsp",
		"百度网盘":               "bdwp",
	}
	for name, want := range cases {
		if got := Initials(name); got != want {
			t.Errorf("Initials(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestInitialsKeepsLatinAndDropsSeparators(t *testing.T) {
	// Letters lower-case, digits kept, separators dropped: a query typed as
	// one word matches across them.
	if got := Initials("Visual Studio Code"); got != "visualstudiocode" {
		t.Errorf("latin = %q", got)
	}
	if got := Initials("7-Zip"); got != "7zip" {
		t.Errorf("digits = %q", got)
	}
	if got := Initials("QQ 音乐"); got != "qqyy" {
		t.Errorf("mixed = %q", got)
	}
}

func TestInitialOfUnknownCharacters(t *testing.T) {
	// A character outside the table contributes nothing, so a name with one
	// keeps the rest of its initials.
	if got := Initial('𠀀'); got != "" {
		t.Errorf("an exotic character = %q", got)
	}
	if got := Initials("A𠀀B"); got != "ab" {
		t.Errorf("a name with an exotic character = %q", got)
	}
}
