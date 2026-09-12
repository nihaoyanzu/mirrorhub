package pypi

import (
	"strings"
	"testing"
)

func TestFilterArtifactsPortable(t *testing.T) {
	urls := []string{
		"https://files.pythonhosted.org/packages/xx/six-1.16.0-py2.py3-none-any.whl",
		"https://files.pythonhosted.org/packages/xx/six-1.16.0.tar.gz",
		"https://files.pythonhosted.org/packages/xx/torch-2.0.0-cp313-cp313-win_amd64.whl",
		"https://files.pythonhosted.org/packages/xx/numpy-1.24.0-cp39-cp39-manylinux_2_17_x86_64.whl",
		"https://files.pythonhosted.org/packages/xx/torch-2.0.0-cp313-cp313-manylinux_2_28_x86_64.whl",
		"https://files.pythonhosted.org/packages/xx/torch-2.0.0-cp313-cp313-macosx_11_0_arm64.whl",
	}

	got := FilterArtifacts(urls, "portable", nil, "linux")
	if len(got) != 2 {
		t.Fatalf("无 tags 应只保留 sdist+none-any，got %d %v", len(got), got)
	}

	got = FilterArtifacts(urls, "portable", []string{"cp313"}, "linux")
	for _, u := range got {
		if strings.Contains(u, "win_amd64") || strings.Contains(u, "macosx") {
			t.Fatalf("linux+cp313 不得保留 win/mac: %s", u)
		}
	}
	hasMany := false
	for _, u := range got {
		if strings.Contains(u, "manylinux") && strings.Contains(u, "cp313") {
			hasMany = true
		}
	}
	if !hasMany {
		t.Fatalf("应保留 manylinux+cp313，got %v", got)
	}

	got = FilterArtifacts(urls, "portable", []string{"manylinux"}, "linux")
	if len(got) < 3 {
		t.Fatalf("manylinux tag 应保留平台 wheel，got %v", got)
	}

	// "all" 模式也应用平台过滤：保留 sdist + none-any + 目标平台 wheel
	all := FilterArtifacts(urls, "all", nil, "linux")
	for _, u := range all {
		if strings.Contains(u, "win_amd64") || strings.Contains(u, "macosx") {
			t.Fatalf("all+linux 不应保留 win/mac wheel: %s", u)
		}
	}
	if len(all) != 4 { // sdist + none-any + manylinux*2
		t.Fatalf("all+linux 应保留 4 个，got %d %v", len(all), all)
	}

	// "all" 模式 + darwin 平台：保留 macosx wheel
	allDarwin := FilterArtifacts(urls, "all", nil, "darwin")
	hasMac := false
	for _, u := range allDarwin {
		if strings.Contains(u, "macosx") {
			hasMac = true
		}
		if strings.Contains(u, "win_amd64") {
			t.Fatalf("all+darwin 不应保留 win wheel: %s", u)
		}
	}
	if !hasMac {
		t.Fatalf("all+darwin 应保留 macosx wheel")
	}

	onlyWin := FilterArtifacts([]string{urls[2]}, "portable", []string{"cp313"}, "linux")
	if len(onlyWin) != 0 {
		t.Fatalf("win wheel 在 linux 目标下应过滤")
	}

	winOK := FilterArtifacts([]string{urls[2]}, "portable", []string{"cp313"}, "win32")
	if len(winOK) != 1 {
		t.Fatalf("win32 目标应保留 win wheel，got %v", winOK)
	}
}

func TestIsPlausiblePackageName(t *testing.T) {
	ok := []string{"requests", "torch", "Pillow", "ruamel.yaml"}
	for _, n := range ok {
		if !IsPlausiblePackageName(n) {
			t.Fatalf("应接受 %q", n)
		}
	}
	bad := []string{"win32", "Linux", "AMD64", "3.11", "3.9", "aarch64", "darwin"}
	for _, n := range bad {
		if IsPlausiblePackageName(n) {
			t.Fatalf("应拒绝 %q", n)
		}
	}
}

func TestSelectFilesBestVersion(t *testing.T) {
	req, err := ParseRequirement("demo>=1.0,<2")
	if err != nil {
		t.Fatal(err)
	}
	files := []string{
		"https://example/demo-1.0.0-py3-none-any.whl",
		"https://example/demo-1.5.0.tar.gz",
		"https://example/demo-1.5.0-py3-none-any.whl",
		"https://example/demo-2.0.0-py3-none-any.whl",
		"https://example/demo-1.5.0a1-py3-none-any.whl",
	}
	ver, urls, err := SelectFiles(files, req)
	if err != nil {
		t.Fatal(err)
	}
	if ver != "1.5.0" {
		t.Fatalf("应选 1.5.0，got %s", ver)
	}
	if len(urls) != 2 {
		t.Fatalf("1.5.0 应有 2 个文件，got %v", urls)
	}
}

func TestSelectFilesNoMatch(t *testing.T) {
	req, _ := ParseRequirement("demo==9.9.9")
	_, _, err := SelectFiles([]string{"https://example/demo-1.0.0.tar.gz"}, req)
	if err == nil {
		t.Fatal("无匹配应报错")
	}
	_, _, err = SelectFiles([]string{"https://example/not-a-package"}, req)
	if err == nil {
		t.Fatal("无解析版本应报错")
	}
}

func TestComparePrerelease(t *testing.T) {
	a, _ := ParseVersion("1.0.0rc1")
	b, _ := ParseVersion("1.0.0")
	if Compare(a, b) >= 0 {
		t.Fatal("rc < final")
	}
	c, _ := ParseVersion("1.0.0a2")
	if Compare(c, a) >= 0 {
		t.Fatal("a < rc")
	}
}

func TestParseRequirementExtras(t *testing.T) {
	req, err := ParseRequirement("requests[security]>=2.0,<3")
	if err != nil {
		t.Fatal(err)
	}
	if req.Name != "requests" || len(req.Constraints) != 2 {
		t.Fatalf("%+v", req)
	}
	_, err = ParseRequirement("")
	if err == nil {
		t.Fatal("空规格应失败")
	}
}

func TestParseLinkDigest(t *testing.T) {
	hex := strings.Repeat("ab", 32)
	algo, dig, clean := ParseLinkDigest("https://h/p.whl#sha256=" + hex)
	if algo != "sha256" || dig != hex || strings.Contains(clean, "#") {
		t.Fatalf("algo=%s dig=%s clean=%s", algo, dig, clean)
	}
	_, _, clean = ParseLinkDigest("https://h/p.whl#md5=dead")
	if strings.Contains(clean, "#") {
		t.Fatal("非 sha256 fragment 也应剥离 URL")
	}
}

func TestExtractArtifactRefsYanked(t *testing.T) {
	html := []byte(`
<a href="https://files.example/pkg-1.0.0.tar.gz#sha256=` + strings.Repeat("0", 64) + `">pkg</a>
<a href="../pkg-1.0.1-py3-none-any.whl" data-yanked="reason">y</a>
`)
	refs := ExtractArtifactRefs(html, "https://pypi.org/simple/pkg/")
	if len(refs) != 2 {
		t.Fatalf("got %d refs", len(refs))
	}
	if refs[0].SHA256 == "" {
		t.Fatal("应解析 sha256")
	}
	if !refs[1].Yanked {
		t.Fatal("应识别 data-yanked")
	}
	if !strings.HasPrefix(refs[1].URL, "https://pypi.org/simple/") {
		t.Fatalf("相对链接应解析为绝对: %s", refs[1].URL)
	}
}

func TestNormalizeName(t *testing.T) {
	if NormalizeName("Foo_Bar.Baz") != "foo-bar-baz" {
		t.Fatal(NormalizeName("Foo_Bar.Baz"))
	}
}
