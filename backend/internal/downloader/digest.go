package downloader

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// digests 记录索引里见到的 cleanURL / basename -> sha256 hex
var digests sync.Map

var (
	digestPersistPath string
	digestPersistMu   sync.Mutex
	digestDirty       bool
	digestSaveSched   bool
)

// InitDigestPersist 从 cache 目录加载并定期落盘 digests。
func InitDigestPersist(cacheDir string) {
	digestPersistPath = filepath.Join(cacheDir, "digests.json")
	loadDigestsFile()
}

func loadDigestsFile() {
	if digestPersistPath == "" {
		return
	}
	b, err := os.ReadFile(digestPersistPath)
	if err != nil {
		return
	}
	var m map[string]string
	if json.Unmarshal(b, &m) != nil {
		return
	}
	for k, v := range m {
		if k == "" || len(v) != 64 {
			continue
		}
		digests.Store(k, strings.ToLower(v))
	}
}

func scheduleDigestSave() {
	digestPersistMu.Lock()
	digestDirty = true
	if digestSaveSched || digestPersistPath == "" {
		digestPersistMu.Unlock()
		return
	}
	digestSaveSched = true
	digestPersistMu.Unlock()
	go func() {
		time.Sleep(2 * time.Second)
		flushDigests()
	}()
}

func flushDigests() {
	digestPersistMu.Lock()
	defer digestPersistMu.Unlock()
	digestSaveSched = false
	if !digestDirty || digestPersistPath == "" {
		return
	}
	out := map[string]string{}
	digests.Range(func(k, v any) bool {
		ks, _ := k.(string)
		vs, _ := v.(string)
		if ks != "" && vs != "" {
			out[ks] = vs
		}
		return true
	})
	b, err := json.Marshal(out)
	if err != nil {
		return
	}
	tmp := digestPersistPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, digestPersistPath)
	digestDirty = false
}

func RememberDigest(rawURL, sha256hex string) {
	sha256hex = strings.ToLower(strings.TrimSpace(sha256hex))
	if sha256hex == "" || len(sha256hex) != 64 {
		return
	}
	clean := stripFragment(rawURL)
	if clean == "" {
		return
	}
	digests.Store(clean, sha256hex)
	if u, err := url.Parse(clean); err == nil {
		base := path.Base(u.Path)
		if base != "" && base != "." && base != "/" {
			digests.Store(base, sha256hex)
		}
		// 无 query 的 URL 也记一份
		u.RawQuery = ""
		u.Fragment = ""
		digests.Store(u.String(), sha256hex)
	} else if base := path.Base(clean); base != "" {
		digests.Store(base, sha256hex)
	}
	scheduleDigestSave()
}

func LookupDigest(rawURL string) string {
	clean := stripFragment(rawURL)
	if v, ok := digests.Load(clean); ok {
		return v.(string)
	}
	if u, err := url.Parse(clean); err == nil {
		u.RawQuery = ""
		u.Fragment = ""
		if v, ok := digests.Load(u.String()); ok {
			return v.(string)
		}
		base := path.Base(u.Path)
		if v, ok := digests.Load(base); ok {
			return v.(string)
		}
	}
	if base := path.Base(clean); base != "" {
		if v, ok := digests.Load(base); ok {
			return v.(string)
		}
	}
	return ""
}

func stripFragment(raw string) string {
	if i := strings.IndexByte(raw, '#'); i >= 0 {
		return raw[:i]
	}
	return raw
}

func (opt *Options) resolveExpectedDigest() {
	if opt.ExpectedSHA256 != "" {
		return
	}
	if d := LookupDigest(opt.metaSource()); d != "" {
		opt.ExpectedSHA256 = d
		return
	}
	if d := LookupDigest(opt.URL); d != "" {
		opt.ExpectedSHA256 = d
	}
}

func verifyAndRemember(path string, opt Options) error {
	if opt.ExpectedSHA256 == "" {
		return nil
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, opt.ExpectedSHA256) {
		_ = os.Remove(path)
		return fmt.Errorf("sha256 mismatch: want %s got %s", opt.ExpectedSHA256, sum)
	}
	RememberDigest(opt.metaSource(), sum)
	RememberDigest(opt.URL, sum)
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
