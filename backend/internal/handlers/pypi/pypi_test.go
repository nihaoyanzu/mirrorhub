package pypi

import (
	"bytes"
	"strings"
	"testing"

	"github.com/livehl/mirrorhub/internal/config"
)

func testCFG(publicHost string) config.Config {
	return config.Config{
		Server: config.ServerConfig{PublicHost: publicHost},
		Platforms: map[string]config.PlatformConfig{
			"pypi": {
				Upstream:     "https://pypi.org/simple",
				FileUpstream: "https://files.pythonhosted.org",
			},
		},
	}
}

func TestPublicBaseURL(t *testing.T) {
	if PublicBaseURL(config.Config{}) != "http://127.0.0.1" {
		t.Fatal("空 PublicHost")
	}
	cfg := config.Config{Server: config.ServerConfig{PublicHost: "https://mirror.example:18081"}}
	if PublicBaseURL(cfg) != "https://mirror.example:18081" {
		t.Fatal(PublicBaseURL(cfg))
	}
	cfg.Server.PublicHost = "mirror.local:18081"
	if PublicBaseURL(cfg) != "http://mirror.local:18081" {
		t.Fatal(PublicBaseURL(cfg))
	}
}

func TestIndexETagDependsOnBase(t *testing.T) {
	raw := []byte("<a href='x'>")
	a := IndexETag(raw, "https://a")
	b := IndexETag(raw, "https://b")
	same := IndexETag(raw, "https://a")
	if a == b {
		t.Fatal("不同 PublicHost 应产生不同 ETag")
	}
	if a != same {
		t.Fatal("相同输入 ETag 应稳定")
	}
	if !strings.HasPrefix(a, `"`) || !strings.HasSuffix(a, `"`) {
		t.Fatalf("ETag 应带引号: %s", a)
	}
}

func TestRewriteURLsHTMLKeepsDigest(t *testing.T) {
	hex := strings.Repeat("cd", 32)
	body := []byte(`<a href="https://files.pythonhosted.org/packages/ab/six-1.16.0-py2.py3-none-any.whl#sha256=` + hex + `">six</a>`)
	cfg := testCFG("https://gateway.example:18081")
	out := RewriteURLs(body, "text/html", cfg)
	s := string(out)
	if !strings.Contains(s, "<!DOCTYPE html>") {
		t.Fatal("应补 HTML5 doctype")
	}
	if strings.Contains(s, "files.pythonhosted.org") {
		t.Fatalf("应改写上游 host: %s", s)
	}
	if !strings.Contains(s, "https://gateway.example:18081/packages/") {
		t.Fatalf("应指向 PublicHost: %s", s)
	}
	if !strings.Contains(s, "#sha256="+hex) {
		t.Fatalf("应保留 digest fragment: %s", s)
	}
}

func TestRewriteURLsRelativeUnchanged(t *testing.T) {
	body := []byte(`<a href="../packages/xx/pkg.whl">x</a>`)
	cfg := testCFG("https://gateway.example")
	out := RewriteURLs(body, "text/html; charset=utf-8", cfg)
	if !bytes.Contains(out, []byte(`href="../packages/xx/pkg.whl"`)) {
		t.Fatalf("相对链接应保持: %s", out)
	}
}

func TestRewriteURLsJSONSimple(t *testing.T) {
	hex := strings.Repeat("11", 32)
	body := []byte(`{"files":[{"url":"https://files.pythonhosted.org/packages/x/a.whl#sha256=` + hex + `","filename":"a.whl"}]}`)
	cfg := testCFG("https://gw.example")
	out := RewriteURLs(body, "application/vnd.pypi.simple.v1+json", cfg)
	s := string(out)
	if strings.Contains(s, "files.pythonhosted.org") {
		t.Fatalf("json 应改写: %s", s)
	}
	if !strings.Contains(s, "https://gw.example/packages/") || !strings.Contains(s, "#sha256="+hex) {
		t.Fatalf("json 改写不完整: %s", s)
	}
}

func TestEnsureHTML5Document(t *testing.T) {
	out := EnsureHTML5Document([]byte(`<a href="x">x</a>`))
	if !bytes.HasPrefix(bytes.TrimSpace(out), []byte("<!DOCTYPE html>")) {
		t.Fatalf("%s", out)
	}
	jsonBody := []byte(`{"meta":1}`)
	if !bytes.Equal(EnsureHTML5Document(jsonBody), jsonBody) {
		t.Fatal("json 不应包 doctype")
	}
}

func TestRewriteURLsInjectsPEP658Metadata(t *testing.T) {
	cfg := testCFG("https://gateway.example:18081")

	// .whl 链接应被注入 data-dist-info-metadata="true"
	hex := strings.Repeat("ab", 32)
	body := []byte(`<a href="https://files.pythonhosted.org/packages/ab/pkg-1.0-cp312-cp312-linux.whl#sha256=` + hex + `">pkg</a>`)
	out := RewriteURLs(body, "text/html", cfg)
	s := string(out)
	if !strings.Contains(s, `data-dist-info-metadata="true"`) {
		t.Fatalf("whl 链接应注入 data-dist-info-metadata: %s", s)
	}

	// 非 .whl 链接不应注入
	body2 := []byte(`<a href="https://files.pythonhosted.org/packages/ab/pkg-1.0.tar.gz">pkg</a>`)
	out2 := RewriteURLs(body2, "text/html", cfg)
	s2 := string(out2)
	if strings.Contains(s2, "data-dist-info-metadata") {
		t.Fatalf("tar.gz 链接不应注入 data-dist-info-metadata: %s", s2)
	}

	// 上游已有的 data-dist-info-metadata 应被保留（不重复注入）
	body3 := []byte(`<a href="https://files.pythonhosted.org/packages/ab/pkg-1.0-none-any.whl" data-dist-info-metadata="sha256=abc">pkg</a>`)
	out3 := RewriteURLs(body3, "text/html", cfg)
	s3 := string(out3)
	// 应该只有一个 data-dist-info-metadata（不重复）
	if strings.Count(s3, "data-dist-info-metadata") != 1 {
		t.Fatalf("不应重复注入: %s", s3)
	}
}
