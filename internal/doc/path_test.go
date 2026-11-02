package doc

import "testing"

func TestClean(t *testing.T) {
	cases := map[string]bool{
		"":                 true,
		"guide/intro.md":   true,
		"guide/./intro.md": true,
		"../secret.md":     false,
		"/etc/passwd":      false,
		"guide/../../a.md": false,
		"guide\\intro.md":  false,
		"guide//intro.md":  true,
		"guide/\x00bad.md": false,
	}
	for input, valid := range cases {
		_, err := Clean(input)
		if valid && err != nil {
			t.Errorf("Clean(%q) 期望合法，实际报错 %v", input, err)
		}
		if !valid && err == nil {
			t.Errorf("Clean(%q) 期望非法，实际通过", input)
		}
	}
}

func TestVisible(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name   string
		public *bool
		global bool
		draft  bool
		logged bool
		want   bool
	}{
		{"继承全局可读", nil, true, false, false, true},
		{"继承全局不可读", nil, false, false, false, false},
		{"文档覆盖为私有", &no, true, false, false, false},
		{"文档覆盖为公开", &yes, false, false, false, true},
		{"草稿匿名不可见", &yes, true, true, false, false},
		{"草稿登录可见", &yes, true, true, true, true},
		{"登录可见私有文档", &no, true, false, true, true},
	}
	for _, tc := range tests {
		node := &Node{Type: "doc", Draft: tc.draft, Private: !effectivePublic(tc.public, tc.global)}
		if got := Visible(node, tc.logged); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestParseFrontMatter(t *testing.T) {
	input := []byte("---\ntitle: 快速开始\npublic: false\norder: 3\n---\n\n正文内容\n")
	fm := Parse(input)
	if fm.Meta.Title != "快速开始" || fm.Meta.Public == nil || *fm.Meta.Public {
		t.Fatalf("front matter 解析错误: %+v", fm.Meta)
	}
	if fm.Meta.Order != 3 {
		t.Fatalf("order 解析错误: %d", fm.Meta.Order)
	}
	if string(fm.Body) != "正文内容\n" {
		t.Fatalf("正文解析错误: %q", fm.Body)
	}

	round := Parse(Compose(fm.Raw, fm.Body))
	if string(round.Body) != "正文内容\n" || round.Meta.Title != "快速开始" {
		t.Fatalf("往返失败: %+v %q", round.Meta, round.Body)
	}
}

func TestMergeFrontMatter(t *testing.T) {
	raw := []byte("title: 旧标题\ntags: [a, b]\n")
	merged := Merge(raw, map[string]any{"title": "新标题", "public": false})
	fm := Parse(Compose(merged, []byte("正文\n")))
	if fm.Meta.Title != "新标题" {
		t.Fatalf("标题未更新: %+v", fm.Meta)
	}
	if fm.Meta.Public == nil || *fm.Meta.Public {
		t.Fatalf("可见性未更新: %+v", fm.Meta.Public)
	}
}
