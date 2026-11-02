package doc

import (
	"bytes"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type Meta struct {
	Title  string `yaml:"title"`
	Order  int    `yaml:"order"`
	Public *bool  `yaml:"public"`
	Draft  bool   `yaml:"draft"`
}

type FrontMatter struct {
	Meta Meta
	Raw  []byte
	Body []byte
}

func Parse(content []byte) FrontMatter {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return FrontMatter{Body: content}
	}

	rest := normalized[4:]
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return FrontMatter{Body: content}
	}
	raw := rest[:end]

	after := rest[end+4:]
	after = bytes.TrimPrefix(after, []byte("\n"))
	if bytes.HasPrefix(after, []byte("\n")) {
		after = after[1:]
	}

	var meta Meta
	_ = yaml.Unmarshal(raw, &meta)
	return FrontMatter{Meta: meta, Raw: raw, Body: after}
}

func Compose(raw, body []byte) []byte {
	if len(bytes.TrimSpace(raw)) == 0 {
		return body
	}
	out := make([]byte, 0, len(raw)+len(body)+16)
	out = append(out, "---\n"...)
	out = append(out, bytes.TrimRight(raw, "\n")...)
	out = append(out, "\n---\n\n"...)
	out = append(out, body...)
	return out
}

func Title(p string, meta Meta) string {
	if strings.TrimSpace(meta.Title) != "" {
		return meta.Title
	}
	base := path.Base(p)
	base = strings.TrimSuffix(base, path.Ext(base))
	if strings.EqualFold(base, "readme") {
		dir := path.Base(path.Dir(p))
		if dir == "." || dir == "/" {
			return "首页"
		}
		return dir
	}
	return base
}

func IsDoc(p string) bool {
	return strings.EqualFold(path.Ext(p), ".md")
}

func Merge(raw []byte, updates map[string]any) []byte {
	values := map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		_ = yaml.Unmarshal(raw, &values)
	}
	for key, value := range updates {
		if value == nil {
			delete(values, key)
			continue
		}
		values[key] = value
	}
	out, err := yaml.Marshal(values)
	if err != nil {
		return raw
	}
	return out
}

func MarshalOrder(order []string) []byte {
	payload := struct {
		Order []string `yaml:"order"`
	}{Order: order}
	out, err := yaml.Marshal(payload)
	if err != nil {
		return []byte("order: []\n")
	}
	return out
}
