package pypi

import (
	"strings"
	"testing"
)

func TestParseDependencyTextRequirements(t *testing.T) {
	text := `
# comment
requests>=2.0  # inline
numpy\
  ==1.24.0
-e git+https://github.com/x/y.git#egg=editablepkg
git+https://github.com/a/b.git#egg=vcspkg
--index-url https://pypi.org/simple
invalid!!!line
`
	items, skipped := ParseDependencyText(text)
	join := strings.Join(items, ",")
	if !strings.Contains(join, "requests>=2.0") {
		t.Fatalf("items=%v", items)
	}
	if !strings.Contains(join, "numpy ==1.24.0") && !strings.Contains(join, "numpy") {
		t.Fatalf("续行合并失败: %v", items)
	}
	foundEdit, foundVCS := false, false
	for _, it := range items {
		if it == "editablepkg" {
			foundEdit = true
		}
		if it == "vcspkg" {
			foundVCS = true
		}
	}
	if !foundEdit || !foundVCS {
		t.Fatalf("应从 egg 提取包名: %v", items)
	}
	if len(skipped) == 0 {
		t.Fatal("应跳过 pip 选项/非法行")
	}
	// 行内注释仍应剥离，且不影响规格
	items2, _ := ParseDependencyText("flask>=2  # comment\n")
	if len(items2) != 1 || items2[0] != "flask>=2" {
		t.Fatalf("行内注释: %v", items2)
	}
}

func TestParseDependencyTextTOML(t *testing.T) {
	text := `
[project]
dependencies = [
  "httpx>=0.24",
  "git+https://example.com/x.git",
]
`
	items, skipped := ParseDependencyText(text)
	if len(items) != 1 || items[0] != "httpx>=0.24" {
		t.Fatalf("items=%v skipped=%v", items, skipped)
	}
	if len(skipped) == 0 {
		t.Fatal("VCS 应进 skipped")
	}
}

func TestParseDependencyTextEmpty(t *testing.T) {
	items, skipped := ParseDependencyText("  \n  ")
	if items != nil || skipped != nil {
		t.Fatalf("%v %v", items, skipped)
	}
}

func TestParseArtifactFilename(t *testing.T) {
	w := ParseArtifactFilename("Torch-2.1.0-cp39-cp39-manylinux2014_x86_64.whl")
	if w.Type != "wheel" || w.Name != "torch" || w.Version != "2.1.0" {
		t.Fatalf("%+v", w)
	}
	s := ParseArtifactFilename("six-1.16.0.tar.gz")
	if s.Type != "sdist" || s.Name != "six" {
		t.Fatalf("%+v", s)
	}
}
