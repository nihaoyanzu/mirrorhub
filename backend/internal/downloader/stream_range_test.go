package downloader

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSingleRange(t *testing.T) {
	const size int64 = 100

	start, end, ok, err := parseSingleRange("", size)
	if err != nil || ok || start != 0 || end != 0 {
		t.Fatalf("空 Range: %v %v %d-%d", ok, err, start, end)
	}

	start, end, ok, err = parseSingleRange("bytes=0-9", size)
	if err != nil || !ok || start != 0 || end != 9 {
		t.Fatalf("bytes=0-9: %v %v %d-%d", ok, err, start, end)
	}

	start, end, ok, err = parseSingleRange("bytes=50-", size)
	if err != nil || !ok || start != 50 || end != 99 {
		t.Fatalf("open end: %v %v %d-%d", ok, err, start, end)
	}

	start, end, ok, err = parseSingleRange("bytes=-10", size)
	if err != nil || !ok || start != 90 || end != 99 {
		t.Fatalf("suffix: %v %v %d-%d", ok, err, start, end)
	}

	_, _, _, err = parseSingleRange("bytes=0-9,10-19", size)
	if err == nil {
		t.Fatal("多 Range 应报错")
	}

	_, _, _, err = parseSingleRange("bytes=100-110", size)
	if err == nil {
		t.Fatal("越界 start 应报错")
	}

	start, end, ok, err = parseSingleRange("bytes=90-999", size)
	if err != nil || !ok || start != 90 || end != 99 {
		t.Fatalf("end 截断: %v %v %d-%d", ok, err, start, end)
	}

	_, _, _, err = parseSingleRange("items=0-1", size)
	if err == nil {
		t.Fatal("非 bytes 单位应报错")
	}

	_, _, ok, err = parseSingleRange("bytes=0-9", 0)
	if err != nil || ok {
		t.Fatal("size<=0 应视为无 Range")
	}
}

func TestDigestBasenameFallbackAndPersist(t *testing.T) {
	dir := t.TempDir()
	InitDigestPersist(dir)

	hexSum := strings.Repeat("ab", 32)
	url := "https://files.example/packages/aa/bb/unique-digest-pkg-1.0.0-py3-none-any.whl?download=1"
	RememberDigest(url+"#sha256="+hexSum, hexSum)

	if got := LookupDigest(url); got != hexSum {
		t.Fatalf("完整 URL lookup: %q", got)
	}
	if got := LookupDigest("https://other.example/path/unique-digest-pkg-1.0.0-py3-none-any.whl"); got != hexSum {
		t.Fatalf("basename 回退: %q", got)
	}

	// 强制落盘并模拟重启加载
	flushDigests()
	path := filepath.Join(dir, "digests.json")
	if _, err := os.Stat(path); err != nil {
		// schedule 可能尚未标记 dirty 路径；再 Remember 一次后 flush
		RememberDigest(url, hexSum)
		flushDigests()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 {
		t.Fatal("digests.json 应非空")
	}

	// 清空内存表：用不同 persist 路径重新 Init 只加载文件内容（旧 key 仍在 sync.Map）
	// 验证文件内容可被二次 load 覆盖/合并
	dir2 := t.TempDir()
	reload := map[string]string{
		"reload-only-pkg.whl": strings.Repeat("cd", 32),
	}
	raw, _ := json.Marshal(reload)
	if err := os.WriteFile(filepath.Join(dir2, "digests.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	InitDigestPersist(dir2)
	if got := LookupDigest("https://x/reload-only-pkg.whl"); got != strings.Repeat("cd", 32) {
		t.Fatalf("从文件加载失败: %q", got)
	}
}

func TestVerifyAndRememberMismatchRemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob")
	content := []byte("hello-digest")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	good := hex.EncodeToString(sum[:])
	bad := strings.Repeat("0", 64)

	err := verifyAndRemember(path, Options{URL: "https://x/a.whl", ExpectedSHA256: bad})
	if err == nil {
		t.Fatal("错 digest 应失败")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("校验失败应删除临时文件")
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyAndRemember(path, Options{URL: "https://x/a.whl", ExpectedSHA256: good}); err != nil {
		t.Fatal(err)
	}
	if LookupDigest("https://x/a.whl") != good {
		t.Fatal("成功校验应 RememberDigest")
	}
	// 避免与并行测试抢 schedule；给一点时间无强依赖
	_ = time.Second
}

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte("abc"))
	if sum != hex.EncodeToString(want[:]) {
		t.Fatalf("got %s", sum)
	}
}
