// Package npm 提供 npm registry 路径识别与 lockfile 解析。
package npm

import (
	"net/url"
	"strings"
)

// IsTarballPath 判断是否为 registry 制品路径（含 scoped）。
func IsTarballPath(path string) bool {
	path = normalizePath(path)
	if !strings.Contains(path, "/-/") {
		return false
	}
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.gz")
}

// IsReservedProxyPath 非 npm 业务路径，避免抢走 PyPI / 健康检查等。
func IsReservedProxyPath(path string) bool {
	path = normalizePath(path)
	switch {
	case path == "/" || path == "/health":
		return true
	case strings.HasPrefix(path, "/simple"), strings.HasPrefix(path, "/packages"):
		return true
	case strings.HasPrefix(path, "/api/"):
		return true
	case strings.HasPrefix(path, "/v2/") || path == "/v2":
		return true
	case strings.Contains(path, "/@v/") || strings.HasSuffix(path, "/@latest") || strings.HasPrefix(path, "/sumdb/"):
		// Go module proxy / sumdb，避免 npm 抢路由
		return true
	default:
		return false
	}
}

// IsTarballURL 判断绝对 URL 是否像 npm tarball（预取选平台用）。
func IsTarballURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Path == "" {
		return false
	}
	return IsTarballPath(u.Path)
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
