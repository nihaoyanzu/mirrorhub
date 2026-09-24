package maven

import (
	"encoding/xml"
	"regexp"
	"strings"
)

// Coordinate Maven GAV 坐标。
type Coordinate struct {
	GroupID    string
	ArtifactID string
	Version    string
	Scope      string
	Packaging  string // 默认 jar；pom 表示仅元数据
}

// String 返回 group:artifact:version。
func (c Coordinate) String() string {
	return c.GroupID + ":" + c.ArtifactID + ":" + c.Version
}

var (
	gavLineRe   = regexp.MustCompile(`(?i)^([A-Za-z0-9_.-]+):([A-Za-z0-9_.-]+):([A-Za-z0-9._+-]+)$`)
	xmlnsAttrRe = regexp.MustCompile(`\sxmlns(:[A-Za-z0-9_]+)?="[^"]*"`)
	propRefRe   = regexp.MustCompile(`\$\{([A-Za-z0-9_.-]+)\}`)
)

// LookLikePom 粗判是否为 pom.xml 内容。
func LookLikePom(text string) bool {
	s := strings.TrimSpace(text)
	if s == "" {
		return false
	}
	head := s
	if len(head) > 800 {
		head = head[:800]
	}
	low := strings.ToLower(head)
	if strings.Contains(low, "<project") && (strings.Contains(low, "maven.apache.org") ||
		strings.Contains(low, "<artifactid") || strings.Contains(low, "<groupid")) {
		return true
	}
	return strings.Contains(low, "<dependencies>") && strings.Contains(low, "<dependency>")
}

// LookLikeGAVList 粗判是否为多行 g:a:v（Gradle/Maven 坐标列表）。
func LookLikeGAVList(text string) bool {
	s := strings.TrimSpace(text)
	if s == "" || LookLikePom(s) {
		return false
	}
	lines, hits := 0, 0
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		lines++
		if _, ok := ParseCoordinate(line); ok {
			hits++
		}
	}
	return lines > 0 && hits*2 >= lines
}

// ParseCoordinate 解析单行 group:artifact:version（可带 @jar 包装忽略）。
func ParseCoordinate(raw string) (Coordinate, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "//") {
		return Coordinate{}, false
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return Coordinate{}, false
	}
	// 去掉 Gradle 引号与尾部分类器简写以外的噪音
	raw = strings.Trim(raw, `"' `)
	if i := strings.IndexAny(raw, " \t"); i >= 0 {
		raw = raw[:i]
	}
	m := gavLineRe.FindStringSubmatch(raw)
	if m == nil {
		return Coordinate{}, false
	}
	return Coordinate{
		GroupID:    m[1],
		ArtifactID: m[2],
		Version:    m[3],
		Packaging:  "jar",
	}, true
}

// ParseGAVList 解析多行坐标列表。
func ParseGAVList(text string) (coords []Coordinate, skipped []string) {
	seen := map[string]struct{}{}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		c, ok := ParseCoordinate(line)
		if !ok {
			skipped = append(skipped, line)
			continue
		}
		key := c.String()
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		coords = append(coords, c)
	}
	return coords, skipped
}

// pomXML 精简 POM 结构。
type pomXML struct {
	XMLName      xml.Name        `xml:"project"`
	Packaging    string          `xml:"packaging"`
	Properties   pomProperties   `xml:"properties"`
	Dependencies pomDependencies `xml:"dependencies"`
}

type pomProperties struct {
	Raw []byte `xml:",innerxml"`
}

type pomDependencies struct {
	Dependency []pomDependency `xml:"dependency"`
}

type pomDependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
	Optional   string `xml:"optional"`
	Type       string `xml:"type"`
}

// ParsePomDependencies 解析 pom.xml 的 <dependencies>（跳过 test / provided / system / optional）。
// 会剥 xmlns，并用 <properties> 做一层 ${name} 替换。
func ParsePomDependencies(text string) (coords []Coordinate, skipped []string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	text = stripXMLNS(text)
	var pom pomXML
	if err := xml.Unmarshal([]byte(text), &pom); err != nil {
		if block := extractTopLevelDependenciesBlock(text); block != "" {
			var deps pomDependencies
			if err2 := xml.Unmarshal([]byte("<dependencies>"+block+"</dependencies>"), &deps); err2 != nil {
				return nil, []string{"pom parse error: " + err.Error()}
			}
			pom.Dependencies = deps
		} else {
			return nil, []string{"pom parse error: " + err.Error()}
		}
	}
	props := parsePropertyMap(pom.Properties.Raw)
	// 常见内置：project.version 等无法可靠解析时保持占位并跳过
	seen := map[string]struct{}{}
	for _, d := range pom.Dependencies.Dependency {
		g := resolveProps(strings.TrimSpace(d.GroupID), props)
		a := resolveProps(strings.TrimSpace(d.ArtifactID), props)
		v := resolveProps(strings.TrimSpace(d.Version), props)
		if g == "" || a == "" || v == "" {
			skipped = append(skipped, g+":"+a+":"+v)
			continue
		}
		if strings.Contains(v, "${") || strings.Contains(g, "${") || strings.Contains(a, "${") {
			skipped = append(skipped, g+":"+a+":"+v)
			continue
		}
		scope := strings.ToLower(strings.TrimSpace(d.Scope))
		if scope == "test" || scope == "provided" || scope == "system" {
			skipped = append(skipped, g+":"+a+":"+v+" ("+scope+")")
			continue
		}
		if strings.EqualFold(strings.TrimSpace(d.Optional), "true") {
			skipped = append(skipped, g+":"+a+":"+v+" (optional)")
			continue
		}
		pack := strings.ToLower(strings.TrimSpace(d.Type))
		if pack == "" {
			pack = "jar"
		}
		c := Coordinate{GroupID: g, ArtifactID: a, Version: v, Scope: scope, Packaging: pack}
		key := c.String()
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		coords = append(coords, c)
	}
	return coords, skipped
}

func stripXMLNS(s string) string {
	return xmlnsAttrRe.ReplaceAllString(s, "")
}

func parsePropertyMap(inner []byte) map[string]string {
	out := map[string]string{}
	if len(inner) == 0 {
		return out
	}
	// <foo>bar</foo>
	re := regexp.MustCompile(`(?s)<([A-Za-z0-9_.-]+)>([^<]*)</([A-Za-z0-9_.-]+)>`)
	for _, m := range re.FindAllSubmatch(inner, -1) {
		if string(m[1]) != string(m[3]) {
			continue
		}
		out[string(m[1])] = strings.TrimSpace(string(m[2]))
	}
	return out
}

func resolveProps(s string, props map[string]string) string {
	if s == "" || !strings.Contains(s, "${") {
		return s
	}
	return propRefRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := propRefRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		if v, ok := props[sub[1]]; ok && v != "" && !strings.Contains(v, "${") {
			return v
		}
		return m
	})
}

// extractTopLevelDependenciesBlock 取 project 下第一段不在 dependencyManagement/plugin 内的 dependencies。
func extractTopLevelDependenciesBlock(text string) string {
	low := strings.ToLower(text)
	// 去掉 dependencyManagement 段，避免误取其中的 dependencies
	cleaned := text
	if i := strings.Index(low, "<dependencymanagement"); i >= 0 {
		if j := strings.Index(low[i:], "</dependencymanagement>"); j >= 0 {
			end := i + j + len("</dependencymanagement>")
			cleaned = text[:i] + text[end:]
			low = strings.ToLower(cleaned)
		}
	}
	start := strings.Index(low, "<dependencies>")
	end := strings.Index(low, "</dependencies>")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return cleaned[start+len("<dependencies>") : end]
}
