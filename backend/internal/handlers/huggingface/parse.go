package huggingface

import (
	"encoding/json"
	"net/url"
	"strings"
)

// RepoRef 预取用的仓库引用。
type RepoRef struct {
	RepoType string // models | datasets
	ID       string // owner/repo 或单段 id
	Revision string // 默认 main
}

// ParseRepoRef 解析 owner/repo、owner/repo@rev、datasets/owner/repo、hf://、Hub URL。
func ParseRepoRef(raw string) (RepoRef, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RepoRef{}, false
	}
	// 已是制品/API URL → 不走 repo 展开
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil || u.Path == "" {
			return RepoRef{}, false
		}
		return parsePathAsRepo(u.Path, u.Fragment)
	}
	if strings.HasPrefix(raw, "hf://") {
		rest := strings.TrimPrefix(raw, "hf://")
		return parsePathAsRepo("/"+rest, "")
	}
	return parsePathAsRepo("/"+raw, "")
}

func parsePathAsRepo(path, fragment string) (RepoRef, bool) {
	path = normalizePath(path)
	// 去掉 resolve/raw 文件路径，只保留仓库
	for _, marker := range []string{"/resolve/", "/raw/"} {
		if i := strings.Index(path, marker); i > 0 {
			path = path[:i]
			break
		}
	}
	// /api/models/id/... → 取 id
	if strings.HasPrefix(path, "/api/models/") {
		rest := strings.TrimPrefix(path, "/api/models/")
		id, rev := splitIDRevision(rest)
		if id == "" {
			return RepoRef{}, false
		}
		if rev == "" {
			rev = "main"
		}
		return RepoRef{RepoType: "models", ID: id, Revision: rev}, true
	}
	if strings.HasPrefix(path, "/api/datasets/") {
		rest := strings.TrimPrefix(path, "/api/datasets/")
		id, rev := splitIDRevision(rest)
		if id == "" {
			return RepoRef{}, false
		}
		if rev == "" {
			rev = "main"
		}
		return RepoRef{RepoType: "datasets", ID: id, Revision: rev}, true
	}

	repoType := "models"
	if strings.HasPrefix(path, "/datasets/") {
		repoType = "datasets"
		path = strings.TrimPrefix(path, "/datasets/")
	} else {
		path = strings.TrimPrefix(path, "/")
	}
	path = strings.Trim(path, "/")
	if path == "" || strings.Contains(path, " ") {
		return RepoRef{}, false
	}
	// 排除明显非 HF 的路径
	if strings.HasPrefix(path, "v2/") || strings.HasPrefix(path, "simple/") ||
		strings.HasPrefix(path, "packages/") || strings.HasPrefix(path, "sumdb/") {
		return RepoRef{}, false
	}

	id, rev := splitAtRevision(path)
	if id == "" {
		return RepoRef{}, false
	}
	// owner/repo（models）或 datasets 下 1～2 段；拒绝单段裸名以免抢走 PyPI 包名
	parts := strings.Split(id, "/")
	if repoType == "datasets" {
		if len(parts) < 1 || len(parts) > 2 {
			return RepoRef{}, false
		}
	} else if len(parts) != 2 {
		return RepoRef{}, false
	}
	for _, p := range parts {
		if p == "" || strings.ContainsAny(p, "@#?") {
			return RepoRef{}, false
		}
	}
	if rev == "" {
		rev = "main"
	}
	if fragment != "" {
		rev = fragment
	}
	return RepoRef{RepoType: repoType, ID: id, Revision: rev}, true
}

// splitIDRevision 从 api 路径尾部剥 tree/revision 等，保留 repo id。
func splitIDRevision(rest string) (id, rev string) {
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", ""
	}
	// id 可能是 a/b；其后跟 /tree|/revision|/resolve-cache 等
	for _, sep := range []string{"/tree/", "/revision/", "/raw/", "/resolve/"} {
		if i := strings.Index(rest, sep); i > 0 {
			idPart := rest[:i]
			after := rest[i+len(sep):]
			revPart := after
			if j := strings.Index(after, "/"); j >= 0 {
				revPart = after[:j]
			}
			return idPart, revPart
		}
	}
	return splitAtRevision(rest)
}

func splitAtRevision(s string) (id, rev string) {
	if i := strings.LastIndex(s, "@"); i > 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// TreeAPIURL 构造递归 tree 列表 URL。
func TreeAPIURL(upstream, repoType, repoID, revision string) string {
	base := strings.TrimRight(strings.TrimSpace(upstream), "/")
	if base == "" {
		base = "https://huggingface.co"
	}
	if repoType != "datasets" {
		repoType = "models"
	}
	if revision == "" {
		revision = "main"
	}
	return base + "/api/" + repoType + "/" + repoID + "/tree/" + revision + "?recursive=true"
}

// ResolveFileURL 构造文件 resolve URL。
func ResolveFileURL(upstream, repoType, repoID, revision, filePath string) string {
	base := strings.TrimRight(strings.TrimSpace(upstream), "/")
	if base == "" {
		base = "https://huggingface.co"
	}
	filePath = strings.TrimPrefix(filePath, "/")
	if revision == "" {
		revision = "main"
	}
	if repoType == "datasets" {
		return base + "/datasets/" + repoID + "/resolve/" + revision + "/" + filePath
	}
	return base + "/" + repoID + "/resolve/" + revision + "/" + filePath
}

// TreeEntry Hub tree API 条目。
type TreeEntry struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	OID  string `json:"oid"`
}

// ParseTreeEntries 解析 tree JSON（数组）。
func ParseTreeEntries(body []byte) ([]TreeEntry, error) {
	var entries []TreeEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// FilePathsFromTree 仅返回 type=file 的路径。
func FilePathsFromTree(entries []TreeEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.EqualFold(e.Type, "file") && e.Path != "" {
			out = append(out, e.Path)
		}
	}
	return out
}
