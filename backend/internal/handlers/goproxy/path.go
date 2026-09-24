// Package goproxy 提供 Go module proxy（GOPROXY）路径识别与 go.mod/go.sum 解析。
//
// 明确后置（不做）：私有模块鉴权、自建 sumdb、VCS direct 代抓。
package goproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// IsGoproxyPath 是否为模块代理或 sumdb 路径。
func IsGoproxyPath(path string) bool {
	path = normalizePath(path)
	if IsSumDBPath(path) {
		return true
	}
	if strings.HasSuffix(path, "/@latest") {
		return strings.Count(path, "/") >= 2
	}
	return strings.Contains(path, "/@v/")
}

// IsSumDBPath checksum database 代理路径。
func IsSumDBPath(path string) bool {
	path = normalizePath(path)
	return strings.HasPrefix(path, "/sumdb/")
}

// IsSumDBSupported 探活：/sumdb/<db>/supported。
func IsSumDBSupported(path string) bool {
	path = normalizePath(path)
	return strings.HasPrefix(path, "/sumdb/") && strings.HasSuffix(path, "/supported")
}

// IsPackagePath 不可变制品：.mod / .zip。
func IsPackagePath(path string) bool {
	path = normalizePath(path)
	if !strings.Contains(path, "/@v/") {
		return false
	}
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".mod") || strings.HasSuffix(lower, ".zip")
}

// IsIndexPath 可变元数据：list / @latest / .info / sumdb（非 supported）。
func IsIndexPath(path string) bool {
	path = normalizePath(path)
	if IsSumDBSupported(path) {
		return false
	}
	if IsSumDBPath(path) {
		return true
	}
	if strings.HasSuffix(path, "/@latest") {
		return true
	}
	if strings.HasSuffix(path, "/@v/list") {
		return true
	}
	if strings.Contains(path, "/@v/") && strings.HasSuffix(strings.ToLower(path), ".info") {
		return true
	}
	return false
}

// SumDBUpstreamURL 将 /sumdb/sum.golang.org/lookup/... 映射到官方 sum 上游（剥 /sumdb/<db> 前缀）。
func SumDBUpstreamURL(sumBase, path string) string {
	base := strings.TrimRight(strings.TrimSpace(sumBase), "/")
	if base == "" {
		return ""
	}
	path = normalizePath(path)
	rest := path
	if strings.HasPrefix(rest, "/sumdb/") {
		rest = rest[len("/sumdb/"):]
		if i := strings.Index(rest, "/"); i >= 0 {
			rest = rest[i:]
		} else {
			rest = "/"
		}
	}
	if rest == "/supported" || strings.HasSuffix(path, "/supported") {
		return ""
	}
	return base + rest
}

// SumDBFetchURL 选择 sumdb 回源地址：官方 sum 剥前缀；第三方 module proxy 则整路径拼接。
func SumDBFetchURL(sumBase, path string) string {
	base := strings.TrimRight(strings.TrimSpace(sumBase), "/")
	path = normalizePath(path)
	if IsSumDBSupported(path) {
		return ""
	}
	if strings.Contains(strings.ToLower(base), "sum.golang.org") {
		return SumDBUpstreamURL(base, path)
	}
	// goproxy.cn 等：GET {proxy}/sumdb/sum.golang.org/lookup/...
	return base + path
}

// PrefetchLookupURL 完整上游 lookup URL。
func PrefetchLookupURL(sumBase, modulePath, version string) (string, error) {
	escMod, err := EscapePath(modulePath)
	if err != nil {
		return "", err
	}
	escVer, err := EscapeVersion(version)
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(strings.TrimSpace(sumBase), "/")
	if base == "" {
		return "", errInvalid("empty sumdb upstream")
	}
	rel := "/sumdb/sum.golang.org/lookup/" + escMod + "@" + escVer
	if strings.Contains(strings.ToLower(base), "sum.golang.org") {
		return strings.TrimRight(base, "/") + "/lookup/" + escMod + "@" + escVer, nil
	}
	return base + rel, nil
}

// ModuleProxyURLs 构造某 module@version 在模块上游上的 info/mod/zip URL，以及 sumdb lookup。
func ModuleProxyURLs(modBase, sumBase, modulePath, version string) (info, mod, zip, lookup string, err error) {
	escMod, err := EscapePath(modulePath)
	if err != nil {
		return "", "", "", "", err
	}
	escVer, err := EscapeVersion(version)
	if err != nil {
		return "", "", "", "", err
	}
	base := strings.TrimRight(strings.TrimSpace(modBase), "/")
	prefix := base + "/" + escMod + "/@v/" + escVer
	info = prefix + ".info"
	mod = prefix + ".mod"
	zip = prefix + ".zip"
	lookup, err = PrefetchLookupURL(sumBase, modulePath, version)
	return info, mod, zip, lookup, err
}

// EscapePath 模块路径大小写转义（A → !a）。
func EscapePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errInvalid("empty module path")
	}
	return escapeASCII(p)
}

// EscapeVersion 版本大小写转义。
func EscapeVersion(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", errInvalid("empty version")
	}
	return escapeASCII(v)
}

func escapeASCII(s string) (string, error) {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			b.WriteByte('!')
			b.WriteByte(c - 'A' + 'a')
			continue
		}
		b.WriteByte(c)
	}
	return b.String(), nil
}

// IndexETag 由正文生成。
func IndexETag(raw []byte) string {
	sum := sha256.Sum256(raw)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// DetectContentType 为 list/info/mod/sumdb 选 Content-Type。
func DetectContentType(path, upstreamCT string, body []byte) string {
	ct := strings.TrimSpace(strings.Split(upstreamCT, ";")[0])
	path = strings.ToLower(normalizePath(path))
	switch {
	case strings.HasSuffix(path, ".info"), strings.HasSuffix(path, "/@latest"):
		if ct != "" {
			return ct
		}
		return "application/json"
	case strings.HasSuffix(path, ".mod"):
		return "text/plain; charset=utf-8"
	case strings.HasSuffix(path, ".zip"):
		return "application/zip"
	case strings.HasSuffix(path, "/@v/list"):
		return "text/plain; charset=utf-8"
	case strings.Contains(path, "/sumdb/"):
		return "text/plain; charset=utf-8"
	}
	if ct != "" {
		return ct
	}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "{") {
		return "application/json"
	}
	return "text/plain; charset=utf-8"
}

// IsGoproxyArtifactURL 预取 URL 是否属于 goproxy 制品。
func IsGoproxyArtifactURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Path == "" {
		return false
	}
	return IsPackagePath(u.Path) || IsIndexPath(u.Path) || strings.Contains(u.Path, "/sumdb/")
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

type invalidError string

func (e invalidError) Error() string { return string(e) }

func errInvalid(msg string) error { return invalidError(msg) }
