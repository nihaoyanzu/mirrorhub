package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetStaleAfterTTLExpiry(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	// 写入一个 TTL=1 秒的条目
	src := filepath.Join(dir, "src.txt")
	_ = os.WriteFile(src, []byte("hello"), 0o644)
	key := KeyFromURL("https://example/simple/pkg/")
	_, err = m.Put(key, src, "text/html", 1, Meta{
		SourceURL:    "https://example/simple/pkg/",
		Kind:         "index",
		UpstreamETag: `"abc123"`,
	})
	if err != nil {
		t.Fatal(err)
	}

	// TTL 内：Get 命中，GetStale 也命中
	if _, ok := m.Get(key); !ok {
		t.Fatal("TTL 内 Get 应命中")
	}
	if _, ok := m.GetStale(key); !ok {
		t.Fatal("TTL 内 GetStale 应命中")
	}

	// 等待过期
	time.Sleep(1100 * time.Millisecond)

	// 过期后：Get 未命中（条目被删除）
	if _, ok := m.Get(key); ok {
		t.Fatal("过期后 Get 应未命中")
	}

	// GetStale 不检查 TTL，仍能拿到旧条目
	stale, ok := m.GetStale(key)
	if !ok {
		t.Fatal("过期后 GetStale 应命中")
	}
	if stale.UpstreamETag != `"abc123"` {
		t.Fatalf("UpstreamETag=%q", stale.UpstreamETag)
	}
}

func TestGetStaleFileDeleted(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	src := filepath.Join(dir, "src.txt")
	_ = os.WriteFile(src, []byte("data"), 0o644)
	key := KeyFromURL("https://example/simple/pkg2/")
	entry, err := m.Put(key, src, "text/html", 0, Meta{
		SourceURL:    "https://example/simple/pkg2/",
		Kind:         "index",
		UpstreamETag: `"etag2"`,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 删除物理文件
	_ = os.Remove(entry.FilePath)

	// GetStale 应返回 false（物理文件不存在）
	if _, ok := m.GetStale(key); ok {
		t.Fatal("文件被删后 GetStale 应返回 false")
	}
}

func TestRenewTTLMakesGetHitAgain(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	src := filepath.Join(dir, "src.txt")
	_ = os.WriteFile(src, []byte("renew-me"), 0o644)
	key := KeyFromURL("https://example/simple/renew/")
	_, err = m.Put(key, src, "text/html", 1, Meta{
		SourceURL:    "https://example/simple/renew/",
		Kind:         "index",
		UpstreamETag: `"etag-renew"`,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 等待过期
	time.Sleep(1100 * time.Millisecond)
	if _, ok := m.Get(key); ok {
		t.Fatal("过期后 Get 应未命中")
	}

	// 续期 TTL
	if err := m.RenewTTL(key, 300); err != nil {
		t.Fatal(err)
	}

	// 续期后 Get 应重新命中
	entry, ok := m.Get(key)
	if !ok {
		t.Fatal("续期后 Get 应命中")
	}
	if entry.UpstreamETag != `"etag-renew"` {
		t.Fatalf("UpstreamETag=%q", entry.UpstreamETag)
	}
}

func TestRenewTTLNonexistent(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	if err := m.RenewTTL("nonexistent", 300); err == nil {
		t.Fatal("续期不存在的条目应返回错误")
	}
}

func TestUpstreamETagPersisted(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(dir, "src.txt")
	_ = os.WriteFile(src, []byte("persist"), 0o644)
	key := KeyFromURL("https://example/simple/persist/")
	_, err = m.Put(key, src, "text/html", 3600, Meta{
		SourceURL:    "https://example/simple/persist/",
		Kind:         "index",
		UpstreamETag: `"xyz789"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	m.Close()

	// 重新打开（模拟重启）
	m2, err := New(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m2.Close()

	entry, ok := m2.Get(key)
	if !ok {
		t.Fatal("重启后 Get 应命中")
	}
	if entry.UpstreamETag != `"xyz789"` {
		t.Fatalf("重启后 UpstreamETag=%q, 期望 %q", entry.UpstreamETag, `"xyz789"`)
	}
}

func TestUpstreamETagEmptyDefault(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	src := filepath.Join(dir, "src.txt")
	_ = os.WriteFile(src, []byte("no-etag"), 0o644)
	key := KeyFromURL("https://example/simple/noetag/")
	// 不传 UpstreamETag
	entry, err := m.Put(key, src, "text/html", 3600, Meta{
		SourceURL: "https://example/simple/noetag/",
		Kind:      "index",
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.UpstreamETag != "" {
		t.Fatalf("未传 UpstreamETag 时应为空, got %q", entry.UpstreamETag)
	}
}
