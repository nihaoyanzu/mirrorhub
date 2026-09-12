package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPutDigestMismatchDoesNotStore(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	src := filepath.Join(dir, "src.bin")
	body := []byte("cache-put-body")
	if err := os.WriteFile(src, body, 0o644); err != nil {
		t.Fatal(err)
	}
	bad := strings.Repeat("1", 64)
	_, err = m.Put("aabbccddeeff0011", src, "application/octet-stream", 0, Meta{
		SourceURL: "https://example/pkg.whl",
		Kind:      "package",
		Digest:    bad,
	})
	if err == nil {
		t.Fatal("错误 digest 应拒绝入库")
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("校验失败应删除源临时文件")
	}
	if _, ok := m.Get("aabbccddeeff0011"); ok {
		t.Fatal("失败后缓存中不应有条目")
	}
	st := m.Stats()
	if st.Entries != 0 {
		t.Fatalf("entries=%d", st.Entries)
	}
}

func TestPutDigestOK(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	src := filepath.Join(dir, "ok.bin")
	body := []byte("ok-body")
	if err := os.WriteFile(src, body, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	key := KeyFromURL("https://example/ok.whl")
	entry, err := m.Put(key, src, "application/octet-stream", 3600, Meta{
		SourceURL: "https://example/ok.whl",
		Kind:      "package",
		Digest:    digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Digest != digest {
		t.Fatalf("digest=%s", entry.Digest)
	}
	got, ok := m.Get(key)
	if !ok || got.Size != int64(len(body)) {
		t.Fatalf("get ok=%v entry=%+v", ok, got)
	}
}

func TestPutRequiresMeta(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	src := filepath.Join(dir, "x")
	_ = os.WriteFile(src, []byte("x"), 0o644)
	_, err = m.Put("1122334455667788", src, "text/plain", 0, Meta{})
	if err == nil {
		t.Fatal("缺 source_url/kind 应失败")
	}
}
