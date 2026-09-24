package huggingface

import (
	"strings"
)

// LookLikeHFRepoList 判断粘贴文本是否像 HF 仓库列表（owner/repo 或 datasets/...）。
func LookLikeHFRepoList(text string) bool {
	lines := splitNonEmptyLines(text)
	if len(lines) == 0 {
		return false
	}
	ok := 0
	for _, line := range lines {
		line = stripLineComment(line)
		if line == "" {
			continue
		}
		if _, parsed := ParseRepoRef(line); parsed {
			ok++
		}
	}
	return ok > 0 && ok*2 >= len(lines)
}

// ParseRepoList 解析多行仓库引用，返回可预取条目。
func ParseRepoList(text string) (items []string, skipped []string) {
	for _, line := range splitNonEmptyLines(text) {
		raw := line
		line = stripLineComment(line)
		if line == "" {
			continue
		}
		if ref, ok := ParseRepoRef(line); ok {
			item := ref.ID
			if ref.RepoType == "datasets" {
				item = "datasets/" + ref.ID
			}
			if ref.Revision != "" && ref.Revision != "main" {
				item = item + "@" + ref.Revision
			}
			items = append(items, item)
			continue
		}
		skipped = append(skipped, raw)
	}
	return items, skipped
}

func splitNonEmptyLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func stripLineComment(line string) string {
	line = strings.TrimSpace(line)
	if i := strings.Index(line, "#"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	return line
}
