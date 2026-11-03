package showdoc

import (
	"reflect"
	"testing"
)

func TestBuildPlanMapsCatalogsAndHistory(t *testing.T) {
	items := []Item{
		{ItemID: 1, Name: "软件组", Type: 1, AddTime: 1},
		{ItemID: 2, Name: "测试", Type: 5, AddTime: 2},
	}
	catalogs := []Catalog{
		{CatID: 10, ItemID: 1, ParentID: 0, Name: "数据标注", Sort: 1},
		{CatID: 11, ItemID: 1, ParentID: 10, Name: "分类模型", Sort: 1},
	}
	pages := []Page{
		{PageID: 100, ItemID: 1, CatID: 0, Title: "总览", Content: "root", Sort: 1, AddTime: 100},
		{PageID: 101, ItemID: 1, CatID: 10, Title: "说明", Content: "new", Sort: 1, AddTime: 110},
		{PageID: 102, ItemID: 1, CatID: 11, Title: "说明", Content: "nested", Sort: 1, AddTime: 120},
		{PageID: 103, ItemID: 2, CatID: 0, Title: "忽略", Content: "x", Sort: 1, AddTime: 130},
	}
	histories := []History{
		{ID: 1, PageID: 101, Title: "说明", Content: "old", AddTime: 90},
		{ID: 2, PageID: 101, Title: "说明", Content: "new", AddTime: 105},
	}
	users := []User{{UID: 1, Username: "thun", Name: "何政初"}}

	plan := BuildPlan(items, catalogs, pages, histories, users, Options{BotName: "gdocs", History: true})

	var paths []string
	for _, page := range plan.Pages {
		paths = append(paths, page.Path)
	}
	wantPaths := []string{
		"软件组/总览.md",
		"软件组/数据标注/说明.md",
		"软件组/数据标注/分类模型/说明.md",
	}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("paths = %#v, want %#v", paths, wantPaths)
	}

	if plan.Stats.Pages != 3 || plan.Stats.Versions != 4 {
		t.Fatalf("stats = %+v", plan.Stats)
	}
	if len(plan.Pages[1].Versions) != 2 {
		t.Fatalf("history versions = %d", len(plan.Pages[1].Versions))
	}
	if plan.Pages[1].Versions[0].Body != "old" {
		t.Fatalf("first version = %q", plan.Pages[1].Versions[0].Body)
	}

	if got := plan.Meta["软件组"]; !reflect.DeepEqual(got, []string{"数据标注", "总览.md"}) {
		t.Fatalf("group order = %#v", got)
	}
	if got := plan.Meta["软件组/数据标注"]; !reflect.DeepEqual(got, []string{"分类模型", "说明.md"}) {
		t.Fatalf("catalog order = %#v", got)
	}
}

func TestSafeName(t *testing.T) {
	cases := map[string]string{
		"a/b:c*d?":    "a_b_c_d_",
		"  name.  ":   "name",
		"CON":         "_CON",
		"":            "未命名",
		"正常 标题":       "正常 标题",
		"hello%world": "hello_world",
	}
	for input, want := range cases {
		if got := safeName(input); got != want {
			t.Errorf("safeName(%q) = %q, want %q", input, got, want)
		}
	}
}
