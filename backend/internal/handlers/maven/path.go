// Package maven 提供 Maven2 仓库布局路径识别与坐标解析。
//
// 明确后置（不做）：deploy/publish、私仓鉴权、Plugin Portal、多 upstream 按 path 分流、
// SNAPSHOT 强制刷新策略、Ivy / flatDir。
package maven

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path"
	"strings"
)

var artifactExts = map[string]struct{}{
	".pom":    {},
	".jar":    {},
	".aar":    {},
	".war":    {},
	".module": {},
	".zip":    {},
}

var checksumExts = []string{".sha1", ".md5", ".sha256", ".sha512"}

// StripMaven2Prefix 剥可选 /maven2 前缀，返回 Maven2 相对路径。
func StripMaven2Prefix(p string) string {
	p = normalizePath(p)
	if p == "/maven2" || p == "/maven2/" {
		return "/"
	}
	if strings.HasPrefix(p, "/maven2/") {
		return "/" + strings.TrimPrefix(p, "/maven2/")
	}
	return p
}

// IsMavenPath 是否为可代理的 Maven2 路径（metadata 或制品）。
func IsMavenPath(p string) bool {
	p = StripMaven2Prefix(p)
	return IsMetadataPath(p) || IsPackagePath(p)
}

// IsMetadataPath 可变索引：maven-metadata.xml（及同名 checksum）。
func IsMetadataPath(p string) bool {
	p = StripMaven2Prefix(p)
	parts := pathParts(p)
	if len(parts) < 3 {
		return false
	}
	return isMavenMetadataFile(parts[len(parts)-1])
}

// IsPackagePath 不可变制品：pom/jar/aar/war/module/zip（及 checksum）。
func IsPackagePath(p string) bool {
	p = StripMaven2Prefix(p)
	parts := pathParts(p)
	// group…/artifact/version/file → 至少 4 段
	if len(parts) < 4 {
		return false
	}
	file := parts[len(parts)-1]
	if isMavenMetadataFile(file) {
		return false
	}
	base := stripChecksumSuffix(file)
	if !hasArtifactExt(base) {
		return false
	}
	artifact := parts[len(parts)-3]
	if artifact == "" {
		return false
	}
	// 文件名应以 artifactId- 开头（含 classifier / 时间戳 SNAPSHOT）
	return strings.HasPrefix(base, artifact+"-")
}

// IsMavenArtifactURL 预取 URL 是否属于 Maven 制品或 metadata。
func IsMavenArtifactURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Path == "" {
		return false
	}
	return IsMavenPath(u.Path)
}

// TargetURL 拼上游绝对地址（path 已含或不含 /maven2 均可）。
func TargetURL(upstream, reqPath string) string {
	base := strings.TrimRight(strings.TrimSpace(upstream), "/")
	if base == "" {
		base = "https://maven.aliyun.com/repository/central"
	}
	rel := StripMaven2Prefix(reqPath)
	if rel == "/" {
		return base + "/"
	}
	return base + rel
}

// CoordPath 坐标 → Maven2 相对路径前缀（无尾斜杠）：/{group}/{artifact}/{version}
func CoordPath(groupID, artifactID, version string) string {
	g := strings.ReplaceAll(strings.TrimSpace(groupID), ".", "/")
	a := strings.TrimSpace(artifactID)
	v := strings.TrimSpace(version)
	return "/" + path.Join(g, a, v)
}

// ArtifactURL 构造上游制品 URL（默认 packaging=jar 或 pom）。
func ArtifactURL(upstream, groupID, artifactID, version, ext string) string {
	ext = strings.TrimSpace(ext)
	if ext == "" {
		ext = "jar"
	}
	ext = strings.TrimPrefix(ext, ".")
	prefix := CoordPath(groupID, artifactID, version)
	name := strings.TrimSpace(artifactID) + "-" + strings.TrimSpace(version) + "." + ext
	return TargetURL(upstream, prefix+"/"+name)
}

// IndexETag 由正文生成。
func IndexETag(raw []byte) string {
	sum := sha256.Sum256(raw)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// DetectContentType 为 metadata / pom / jar 选 Content-Type。
func DetectContentType(reqPath, upstreamCT string, body []byte) string {
	ct := strings.TrimSpace(strings.Split(upstreamCT, ";")[0])
	p := strings.ToLower(StripMaven2Prefix(reqPath))
	base := stripChecksumSuffix(path.Base(p))
	switch {
	case strings.Contains(p, "maven-metadata.xml"):
		return "application/xml"
	case strings.HasSuffix(base, ".pom"), strings.HasSuffix(base, ".module"):
		return "application/xml"
	case strings.HasSuffix(base, ".jar"), strings.HasSuffix(base, ".aar"),
		strings.HasSuffix(base, ".war"), strings.HasSuffix(base, ".zip"):
		return "application/java-archive"
	}
	if ct != "" {
		return ct
	}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "<") {
		return "application/xml"
	}
	return "application/octet-stream"
}

func isMavenMetadataFile(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "maven-metadata.xml" || strings.HasPrefix(name, "maven-metadata.xml.")
}

func stripChecksumSuffix(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range checksumExts {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

func hasArtifactExt(name string) bool {
	lower := strings.ToLower(name)
	for ext := range artifactExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func pathParts(p string) []string {
	p = strings.Trim(normalizePath(p), "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
