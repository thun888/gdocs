package doc

import (
	"errors"
	"strings"
)

var ErrInvalidPath = errors.New("非法路径")

func Clean(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", nil
	}
	if strings.ContainsAny(p, "\\\x00") {
		return "", ErrInvalidPath
	}
	if strings.HasPrefix(p, "/") {
		return "", ErrInvalidPath
	}

	parts := strings.Split(p, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", ErrInvalidPath
		}
		if len(part) > 255 {
			return "", ErrInvalidPath
		}
		clean = append(clean, part)
	}
	result := strings.Join(clean, "/")
	if len(result) > 1024 {
		return "", ErrInvalidPath
	}
	return result, nil
}

func UnderRoot(root, p string) (string, bool) {
	root = strings.Trim(root, "/")
	if root == "" {
		return p, true
	}
	if p == root {
		return "", false
	}
	if !strings.HasPrefix(p, root+"/") {
		return "", false
	}
	return strings.TrimPrefix(p, root+"/"), true
}
