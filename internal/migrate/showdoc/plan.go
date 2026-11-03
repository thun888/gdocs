package showdoc

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	maxNameRunes = 80
	cstOffset    = 8 * 3600
)

var cst = time.FixedZone("CST", cstOffset)

type Options struct {
	BotName string
	History bool
}

type Version struct {
	Title  string
	Body   string
	Author string
	At     time.Time
}

type PagePlan struct {
	PageID   int64
	Path     string
	Title    string
	Order    int
	Group    string
	Versions []Version
}

type GroupPlan struct {
	ItemID int64
	Path   string
	Name   string
	Pages  int
}

type Stats struct {
	Groups   int
	Pages    int
	Versions int
	Dirs     int
	Meta     int
}

type Plan struct {
	Groups []GroupPlan
	Pages  []*PagePlan
	Meta   map[string][]string
	Stats  Stats
}

func BuildPlan(items []Item, catalogs []Catalog, pages []Page, histories []History, users []User, opt Options) *Plan {
	plan := &Plan{Meta: map[string][]string{}}
	if opt.BotName == "" {
		opt.BotName = "gdocs"
	}

	names := map[int64]string{}
	byUsername := map[string]string{}
	for _, user := range users {
		name := strings.TrimSpace(user.Name)
		if name == "" {
			name = strings.TrimSpace(user.Username)
		}
		if name == "" || strings.EqualFold(name, "showdoc") {
			name = opt.BotName
		}
		names[user.UID] = name
		if user.Username != "" {
			byUsername[user.Username] = name
		}
	}
	authorName := func(uid int64, username string) string {
		if name, ok := names[uid]; ok && name != "" {
			return name
		}
		if name, ok := byUsername[username]; ok && name != "" {
			return name
		}
		if name := strings.TrimSpace(username); name != "" && !strings.EqualFold(name, "showdoc") {
			return name
		}
		return opt.BotName
	}

	active := map[int64]Item{}
	var selected []Item
	for _, item := range items {
		if item.Type != 1 || item.Deleted || item.Archived {
			continue
		}
		active[item.ItemID] = item
		selected = append(selected, item)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].AddTime != selected[j].AddTime {
			return selected[i].AddTime < selected[j].AddTime
		}
		return selected[i].ItemID < selected[j].ItemID
	})

	dirSort := map[string]int{}
	groupPath := map[int64]string{}
	rootUsed := map[string]bool{}
	for i, item := range selected {
		name := uniqueName(safeName(item.Name), rootUsed, item.ItemID)
		dir := name
		groupPath[item.ItemID] = dir
		dirSort[dir] = i
		plan.Groups = append(plan.Groups, GroupPlan{ItemID: item.ItemID, Path: dir, Name: item.Name})
	}

	catsByItem := map[int64][]Catalog{}
	catByID := map[int64]Catalog{}
	for _, cat := range catalogs {
		if _, ok := active[cat.ItemID]; !ok {
			continue
		}
		catsByItem[cat.ItemID] = append(catsByItem[cat.ItemID], cat)
		catByID[cat.CatID] = cat
	}

	catPath := map[int64]string{}
	for itemID := range active {
		list := catsByItem[itemID]
		children := map[int64][]Catalog{}
		for _, cat := range list {
			parent := cat.ParentID
			if _, ok := catByID[parent]; !ok || catByID[parent].ItemID != itemID {
				parent = 0
			}
			children[parent] = append(children[parent], cat)
		}
		var walk func(parent int64, prefix string, depth int)
		walk = func(parent int64, prefix string, depth int) {
			if depth > 32 {
				return
			}
			nodes := children[parent]
			sort.SliceStable(nodes, func(i, j int) bool {
				if nodes[i].Sort != nodes[j].Sort {
					return nodes[i].Sort < nodes[j].Sort
				}
				if nodes[i].AddTime != nodes[j].AddTime {
					return nodes[i].AddTime < nodes[j].AddTime
				}
				return nodes[i].CatID < nodes[j].CatID
			})
			used := map[string]bool{}
			for _, cat := range nodes {
				name := uniqueName(safeName(cat.Name), used, cat.CatID)
				dir := path.Join(prefix, name)
				catPath[cat.CatID] = dir
				dirSort[dir] = cat.Sort
				walk(cat.CatID, dir, depth+1)
			}
		}
		walk(0, groupPath[itemID], 0)
	}

	historyByPage := map[int64][]History{}
	for _, history := range histories {
		historyByPage[history.PageID] = append(historyByPage[history.PageID], history)
	}

	type pageRef struct {
		page Page
		dir  string
	}
	var refs []pageRef
	for _, page := range pages {
		if page.Deleted || page.Draft {
			continue
		}
		group, ok := groupPath[page.ItemID]
		if !ok {
			continue
		}
		dir := group
		if cat, ok := catPath[page.CatID]; ok {
			dir = cat
		}
		refs = append(refs, pageRef{page: page, dir: dir})
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].dir != refs[j].dir {
			return refs[i].dir < refs[j].dir
		}
		if refs[i].page.Sort != refs[j].page.Sort {
			return refs[i].page.Sort < refs[j].page.Sort
		}
		if refs[i].page.AddTime != refs[j].page.AddTime {
			return refs[i].page.AddTime < refs[j].page.AddTime
		}
		return refs[i].page.PageID < refs[j].page.PageID
	})

	usedPaths := map[string]map[string]bool{}
	for _, ref := range refs {
		page := ref.page
		title := strings.TrimSpace(page.Title)
		if title == "" {
			title = "未命名"
		}
		base := stripMarkdownExt(safeName(title))
		if base == "" {
			base = "未命名"
		}
		if usedPaths[ref.dir] == nil {
			usedPaths[ref.dir] = map[string]bool{}
		}
		name := uniqueName(base+".md", usedPaths[ref.dir], page.PageID)

		versions := buildVersions(page, historyByPage[page.PageID], authorName, opt)
		if !opt.History && len(versions) > 1 {
			versions = versions[len(versions)-1:]
		}

		plan.Pages = append(plan.Pages, &PagePlan{
			PageID:   page.PageID,
			Path:     path.Join(ref.dir, name),
			Title:    title,
			Order:    page.Sort,
			Group:    groupPath[page.ItemID],
			Versions: versions,
		})
	}

	plan.Stats.Groups = len(plan.Groups)
	plan.Stats.Pages = len(plan.Pages)
	for _, page := range plan.Pages {
		plan.Stats.Versions += len(page.Versions)
	}
	plan.Meta = buildMeta(plan.Pages, dirSort)
	plan.Stats.Dirs = len(plan.Meta)
	for _, names := range plan.Meta {
		if len(names) > 0 {
			plan.Stats.Meta++
		}
	}
	for i := range plan.Groups {
		for _, page := range plan.Pages {
			if page.Group == plan.Groups[i].Path {
				plan.Groups[i].Pages++
			}
		}
	}
	return plan
}

func buildVersions(page Page, history []History, authorName func(int64, string) string, opt Options) []Version {
	var versions []Version
	for _, item := range history {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = strings.TrimSpace(page.Title)
		}
		versions = append(versions, Version{
			Title:  title,
			Body:   item.Content,
			Author: authorName(item.AuthorUID, item.AuthorName),
			At:     time.Unix(item.AddTime, 0).In(cst),
		})
	}
	sort.SliceStable(versions, func(i, j int) bool { return versions[i].At.Before(versions[j].At) })

	out := make([]Version, 0, len(versions)+1)
	for _, version := range versions {
		if len(out) > 0 && sameVersion(out[len(out)-1], version) {
			continue
		}
		out = append(out, version)
	}

	title := strings.TrimSpace(page.Title)
	if title == "" {
		title = "未命名"
	}
	current := Version{
		Title:  title,
		Body:   page.Content,
		Author: authorName(page.AuthorUID, page.AuthorName),
		At:     time.Unix(page.AddTime, 0).In(cst),
	}
	if len(out) > 0 {
		last := out[len(out)-1]
		if !current.At.After(last.At) {
			current.At = last.At.Add(time.Second)
		}
		if sameVersion(last, current) {
			return out
		}
	}
	return append(out, current)
}

func sameVersion(a, b Version) bool {
	if strings.TrimSpace(a.Title) != strings.TrimSpace(b.Title) {
		return false
	}
	return normalizeBody(a.Body) == normalizeBody(b.Body)
}

func normalizeBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	return strings.TrimRight(body, "\n")
}

func buildMeta(pages []*PagePlan, dirSort map[string]int) map[string][]string {
	type entry struct {
		name string
		sort int
		dir  bool
	}
	entries := map[string]map[string]entry{}
	add := func(dir, name string, rank int, isDir bool) {
		if entries[dir] == nil {
			entries[dir] = map[string]entry{}
		}
		key := strings.ToLower(name)
		if _, ok := entries[dir][key]; ok {
			return
		}
		entries[dir][key] = entry{name: name, sort: rank, dir: isDir}
	}

	for _, page := range pages {
		dir := path.Dir(page.Path)
		if dir == "." {
			dir = ""
		}
		add(dir, path.Base(page.Path), page.Order, false)

		child := dir
		for child != "" {
			parent := path.Dir(child)
			if parent == "." {
				parent = ""
			}
			add(parent, path.Base(child), dirSort[child], true)
			child = parent
		}
	}

	out := make(map[string][]string, len(entries))
	for dir, values := range entries {
		list := make([]entry, 0, len(values))
		for _, item := range values {
			list = append(list, item)
		}
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].sort != list[j].sort {
				return list[i].sort < list[j].sort
			}
			if list[i].dir != list[j].dir {
				return list[i].dir
			}
			return list[i].name < list[j].name
		})
		names := make([]string, 0, len(list))
		for _, item := range list {
			names = append(names, item.name)
		}
		out[dir] = names
	}
	return out
}

func uniqueName(base string, used map[string]bool, suffix int64) string {
	if base == "" {
		base = "未命名"
	}
	key := strings.ToLower(base)
	if !used[key] {
		used[key] = true
		return base
	}
	ext := ""
	if strings.EqualFold(path.Ext(base), ".md") {
		ext = base[len(base)-3:]
		base = base[:len(base)-3]
	}
	name := fmt.Sprintf("%s-%d%s", base, suffix, ext)
	for used[strings.ToLower(name)] {
		suffix++
		name = fmt.Sprintf("%s-%d%s", base, suffix, ext)
	}
	used[strings.ToLower(name)] = true
	return name
}

func stripMarkdownExt(name string) string {
	if strings.EqualFold(path.Ext(name), ".md") {
		return name[:len(name)-3]
	}
	return name
}

func safeName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "未命名"
	}
	raw = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '#', '%':
			return '_'
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, raw)
	raw = strings.Join(strings.Fields(raw), " ")
	raw = strings.Trim(raw, " .")
	runes := []rune(raw)
	if len(runes) > maxNameRunes {
		raw = strings.Trim(string(runes[:maxNameRunes]), " .")
	}
	if raw == "" {
		return "未命名"
	}
	if isReservedName(raw) {
		raw = "_" + raw
	}
	return raw
}

func isReservedName(name string) bool {
	base := name
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	base = strings.ToUpper(strings.TrimSpace(base))
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) == 4 {
		if (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && unicode.IsDigit(rune(base[3])) {
			return true
		}
	}
	return false
}
