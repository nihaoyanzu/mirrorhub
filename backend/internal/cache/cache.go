package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"

	"github.com/livehl/mirrorhub/internal/store"
)

var (
	bucketMeta   = []byte("meta")
	bucketChunks = []byte("chunks")
)

type Entry struct {
	Key           string    `json:"key"`
	FilePath      string    `json:"file_path"`
	ContentType   string    `json:"content_type"`
	Size          int64     `json:"size"`
	CreatedAt     time.Time `json:"created_at"`
	LastAccess    time.Time `json:"last_access"`
	TTLSeconds    int       `json:"ttl_seconds"`
	SourceURL     string    `json:"source_url,omitempty"`
	Kind          string    `json:"kind,omitempty"` // index | package | metadata
	Digest        string    `json:"digest,omitempty"`
	ETag          string    `json:"etag,omitempty"`
	UpstreamETag  string    `json:"upstream_etag,omitempty"` // 上游响应 ETag，用于条件请求续期
}

// Meta 写入缓存时的附加元数据
type Meta struct {
	SourceURL    string
	Kind         string
	Digest       string
	UpstreamETag string // 上游响应 ETag，用于条件请求续期
}

type Stats struct {
	Entries       int     `json:"entries"`
	TotalSize     int64   `json:"total_size"`
	MaxSize       int64   `json:"max_size"`
	Hits          int64   `json:"hits"`
	Misses        int64   `json:"misses"`
	UsageRatio    float64 `json:"usage_ratio"`
	DiskFreeBytes int64   `json:"disk_free_bytes"`
	DiskFreeOK    bool    `json:"disk_free_ok"`
	WaterWarn     bool    `json:"water_warn"`
	WaterCrit     bool    `json:"water_crit"`
}

type Manager struct {
	dir     string
	maxSize int64
	db      *bbolt.DB
	mu      sync.Mutex
	hits    int64
	misses  int64
	dirty   bool
	store   *store.Store
	stop    chan struct{}
	done    chan struct{}
}

const (
	cacheCounterKey = "stats.cache"
	persistEvery    = 5 * time.Second
)

type cacheCounters struct {
	Hits   int64 `json:"hits"`
	Misses int64 `json:"misses"`
}

func New(dir string, maxSizeGB float64, st *store.Store) (*Manager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, "meta.db")
	db, err := bbolt.Open(dbPath, 0o600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		if errors.Is(err, bbolt.ErrTimeout) || strings.Contains(err.Error(), "timeout") {
			return nil, fmt.Errorf("打开缓存库超时（%s 可能被另一个 mirrorhub 进程占用）: %w", dbPath, err)
		}
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketMeta); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(bucketChunks)
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if maxSizeGB <= 0 {
		maxSizeGB = 100
	}
	m := &Manager{
		dir:     dir,
		maxSize: int64(maxSizeGB * float64(1024*1024*1024)),
		db:      db,
		store:   st,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	m.loadCounters()
	go m.persistLoop()
	return m, nil
}

// SetMaxSizeGB 热更新容量上限（支持小数 GiB，便于水位测试）；超限时立即触发淘汰。
func (m *Manager) SetMaxSizeGB(gb float64) {
	if gb <= 0 {
		gb = 100
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxSize = int64(gb * float64(1024*1024*1024))
	if m.maxSize < 1 {
		m.maxSize = 1
	}
	_ = m.evictLocked()
}

func (m *Manager) loadCounters() {
	if m.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var c cacheCounters
	ok, err := m.store.GetJSON(ctx, cacheCounterKey, &c)
	if err != nil || !ok {
		return
	}
	m.mu.Lock()
	m.hits = c.Hits
	m.misses = c.Misses
	m.mu.Unlock()
}

func (m *Manager) saveCounters() {
	if m.store == nil {
		return
	}
	m.mu.Lock()
	if !m.dirty {
		m.mu.Unlock()
		return
	}
	c := cacheCounters{Hits: m.hits, Misses: m.misses}
	m.dirty = false
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = m.store.PutJSON(ctx, cacheCounterKey, c)
}

func (m *Manager) persistLoop() {
	defer close(m.done)
	t := time.NewTicker(persistEvery)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			m.mu.Lock()
			m.dirty = true
			m.mu.Unlock()
			m.saveCounters()
			return
		case <-t.C:
			m.saveCounters()
		}
	}
}

func (m *Manager) Close() error {
	select {
	case <-m.stop:
	default:
		close(m.stop)
	}
	<-m.done
	return m.db.Close()
}

func KeyFromURL(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:16])
}

func (m *Manager) filePath(key string) string {
	sk := safeCacheKey(key)
	if len(sk) < 2 {
		sk = sk + "__"
	}
	return filepath.Join(m.dir, "objects", sk[:2], sk)
}

func (m *Manager) Get(key string) (*Entry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var entry Entry
	err := m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("miss")
		}
		return json.Unmarshal(v, &entry)
	})
	if err != nil {
		m.misses++
		m.dirty = true
		return nil, false
	}
	if strings.TrimSpace(entry.SourceURL) == "" || strings.TrimSpace(entry.Kind) == "" {
		_ = m.deleteLocked(key)
		m.misses++
		m.dirty = true
		return nil, false
	}
		// TTL：index/metadata 按 CreatedAt 过期；package 制品（wheel/sdist）内容不可变，
		// 不按 TTL 失效，仅靠容量淘汰。过期 index 不立即删除，留给 GetStale 条件续期。
		if entry.Kind != "package" && entry.TTLSeconds > 0 && time.Since(entry.CreatedAt) > time.Duration(entry.TTLSeconds)*time.Second {
			m.misses++
			m.dirty = true
			return nil, false
		}
	if _, err := os.Stat(entry.FilePath); err != nil {
		_ = m.deleteLocked(key)
		m.misses++
		m.dirty = true
		return nil, false
	}
	now := time.Now()
	prevAccess := entry.LastAccess
	entry.LastAccess = now
	// 每次 HIT 同步写 bbolt 会把并发吞吐打成串行；LastAccess 最多约 30s 落盘一次（淘汰仍可用）
	if prevAccess.IsZero() || now.Sub(prevAccess) >= 30*time.Second {
		_ = m.putMetaLocked(&entry)
	}
	m.hits++
	m.dirty = true
	return &entry, true
}

// GetStale 返回过期但物理文件仍存在的条目，不检查 TTL、不删除、不更新 LastAccess。
// 用于 revalidation 场景：拿到旧 entry 的 UpstreamETag 向上游发条件请求。
func (m *Manager) GetStale(key string) (*Entry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var entry Entry
	err := m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("miss")
		}
		return json.Unmarshal(v, &entry)
	})
	if err != nil {
		return nil, false
	}
	if strings.TrimSpace(entry.SourceURL) == "" || strings.TrimSpace(entry.Kind) == "" {
		return nil, false
	}
	if _, err := os.Stat(entry.FilePath); err != nil {
		return nil, false
	}
	return &entry, true
}

// RenewTTL 更新条目的 CreatedAt、TTLSeconds 和 LastAccess（不改文件），实现条件请求续期。
func (m *Manager) RenewTTL(key string, ttlSeconds int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var entry Entry
	err := m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("miss")
		}
		return json.Unmarshal(v, &entry)
	})
	if err != nil {
		return err
	}
	entry.CreatedAt = time.Now()
	entry.LastAccess = time.Now()
	entry.TTLSeconds = ttlSeconds
	return m.putMetaLocked(&entry)
}

func (m *Manager) Put(key, srcPath, contentType string, ttlSeconds int, meta Meta) (*Entry, error) {
	if strings.TrimSpace(meta.SourceURL) == "" || strings.TrimSpace(meta.Kind) == "" {
		return nil, fmt.Errorf("cache meta requires source_url and kind")
	}
	if meta.Digest != "" {
		sum, err := fileSHA256Hex(srcPath)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(sum, meta.Digest) {
			_ = os.Remove(srcPath)
			return nil, fmt.Errorf("sha256 mismatch: want %s got %s", meta.Digest, sum)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	dst := m.filePath(key)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}
	_ = os.Remove(dst)
	if err := moveFile(srcPath, dst); err != nil {
		return nil, err
	}
	info, err := os.Stat(dst)
	if err != nil {
		return nil, err
	}
	etag := ""
	if meta.Digest != "" {
		etag = `"` + strings.ToLower(meta.Digest) + `"`
	} else {
		etag = fmt.Sprintf(`W/"%d-%d"`, info.Size(), time.Now().Unix())
	}
	entry := &Entry{
		Key:          key,
		FilePath:     dst,
		ContentType:  contentType,
		Size:         info.Size(),
		CreatedAt:    time.Now(),
		LastAccess:   time.Now(),
		TTLSeconds:   ttlSeconds,
		SourceURL:    meta.SourceURL,
		Kind:         meta.Kind,
		Digest:       strings.ToLower(meta.Digest),
		ETag:         etag,
		UpstreamETag: meta.UpstreamETag,
	}
	if err := m.putMetaLocked(entry); err != nil {
		return nil, err
	}
	_ = m.evictLocked()
	return entry, nil
}

// moveFile 优先 rename，跨设备失败则流式拷贝（避免整文件读入内存）
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}
	_ = os.Remove(src)
	return nil
}

func (m *Manager) PutBytes(key string, body []byte, contentType string, ttlSeconds int, meta Meta) (*Entry, error) {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return nil, err
	}
	tmp := filepath.Join(m.dir, "tmp_"+safeCacheKey(key))
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return nil, err
	}
	defer os.Remove(tmp)
	return m.Put(key, tmp, contentType, ttlSeconds, meta)
}

// safeCacheKey 将任意 cache key 映射为固定 hex 路径段（与宿主机文件系统无关）。
func safeCacheKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:16])
}

func (m *Manager) TempPath(key string) string {
	return filepath.Join(m.dir, "tmp_"+safeCacheKey(key)+".part")
}

func (m *Manager) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deleteLocked(key)
}

func (m *Manager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keys []string
	_ = m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		return b.ForEach(func(k, _ []byte) error {
			keys = append(keys, string(k))
			return nil
		})
	})
	for _, k := range keys {
		_ = m.deleteLocked(k)
	}
	m.hits = 0
	m.misses = 0
	m.dirty = true
	return nil
}

func (m *Manager) Stats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	var n int
	_ = m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		return b.ForEach(func(_, v []byte) error {
			var e Entry
			if json.Unmarshal(v, &e) == nil {
				total += e.Size
				n++
			}
			return nil
		})
	})
	ratio := 0.0
	if m.maxSize > 0 {
		ratio = float64(total) / float64(m.maxSize)
	}
	free, ok := diskFreeBytes(m.dir)
	return Stats{
		Entries:       n,
		TotalSize:     total,
		MaxSize:       m.maxSize,
		Hits:          m.hits,
		Misses:        m.misses,
		UsageRatio:    ratio,
		DiskFreeBytes: free,
		DiskFreeOK:    ok,
		WaterWarn:     ratio >= 0.80,
		WaterCrit:     ratio >= 0.92,
	}
}

// AllowPrefetchWrite 磁盘/容量临界时拒绝预取写入
func (m *Manager) AllowPrefetchWrite() error {
	st := m.Stats()
	if st.WaterCrit {
		return fmt.Errorf("cache water critical (usage=%.1f%%)", st.UsageRatio*100)
	}
	if st.DiskFreeOK && st.DiskFreeBytes < 1<<30 {
		return fmt.Errorf("disk free too low: %d bytes", st.DiskFreeBytes)
	}
	return nil
}

// List 返回全部缓存条目（按最近访问倒序）
func (m *Manager) List() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Entry
	_ = m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		return b.ForEach(func(_, v []byte) error {
			var e Entry
			if json.Unmarshal(v, &e) == nil {
				out = append(out, e)
			}
			return nil
		})
	})
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastAccess.After(out[j].LastAccess)
	})
	return out
}

// PurgeIncomplete 删除缺少 SourceURL 或 Kind 的旧缓存条目及其文件。
func (m *Manager) PurgeIncomplete() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var keys []string
	_ = m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		return b.ForEach(func(k, v []byte) error {
			var e Entry
			if json.Unmarshal(v, &e) != nil {
				keys = append(keys, string(k))
				return nil
			}
			if strings.TrimSpace(e.SourceURL) == "" || strings.TrimSpace(e.Kind) == "" {
				keys = append(keys, e.Key)
				if e.Key == "" {
					keys[len(keys)-1] = string(k)
				}
			}
			return nil
		})
	})
	for _, key := range keys {
		if err := m.deleteLocked(key); err != nil {
			return 0, err
		}
	}
	return len(keys), nil
}

// PurgeTempFiles 清理缓存目录下孤儿临时文件（tmp_*）
func (m *Manager) PurgeTempFiles() (int, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "tmp_") {
			continue
		}
		if err := os.Remove(filepath.Join(m.dir, name)); err == nil {
			n++
		}
	}
	return n, nil
}

type ChunkMark struct {
	URL       string    `json:"url"`
	TotalSize int64     `json:"total_size"`
	Start     int64     `json:"start"`
	End       int64     `json:"end"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *Manager) MarkChunks(url string, totalSize int64, ranges [][2]int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChunks)
		for _, r := range ranges {
			mark := ChunkMark{
				URL: url, TotalSize: totalSize, Start: r[0], End: r[1], UpdatedAt: time.Now(),
			}
			key := chunkKey(url, r[0], r[1])
			data, _ := json.Marshal(mark)
			if err := b.Put([]byte(key), data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *Manager) GetChunks(url string, totalSize int64, ttlHours int) [][2]int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out [][2]int64
	cutoff := time.Now().Add(-time.Duration(ttlHours) * time.Hour)
	_ = m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChunks)
		return b.ForEach(func(_, v []byte) error {
			var mark ChunkMark
			if json.Unmarshal(v, &mark) != nil {
				return nil
			}
			if mark.URL != url || mark.TotalSize != totalSize {
				return nil
			}
			if mark.UpdatedAt.Before(cutoff) {
				return nil
			}
			out = append(out, [2]int64{mark.Start, mark.End})
			return nil
		})
	})
	return out
}

func (m *Manager) ClearChunks(url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChunks)
		var keys [][]byte
		_ = b.ForEach(func(k, v []byte) error {
			var mark ChunkMark
			if json.Unmarshal(v, &mark) == nil && mark.URL == url {
				keys = append(keys, append([]byte{}, k...))
			}
			return nil
		})
		for _, k := range keys {
			_ = b.Delete(k)
		}
		return nil
	})
}

func chunkKey(url string, start, end int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", url, start, end)))
	return hex.EncodeToString(sum[:16])
}

func (m *Manager) putMetaLocked(entry *Entry) error {
	return m.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return b.Put([]byte(entry.Key), data)
	})
}

func (m *Manager) deleteLocked(key string) error {
	var entry Entry
	_ = m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		v := b.Get([]byte(key))
		if v != nil {
			_ = json.Unmarshal(v, &entry)
		}
		return nil
	})
	if entry.FilePath != "" {
		_ = os.Remove(entry.FilePath)
	}
	return m.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketMeta).Delete([]byte(key))
	})
}

func (m *Manager) evictLocked() error {
	type item struct {
		key  string
		e    Entry
	}
	var items []item
	var total int64
	_ = m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		return b.ForEach(func(k, v []byte) error {
			var e Entry
			if json.Unmarshal(v, &e) == nil {
				items = append(items, item{key: string(k), e: e})
				total += e.Size
			}
			return nil
		})
	})
	if total <= m.maxSize {
		return nil
	}
	target := m.maxSize
	// 超过 80% 时多腾出一些空间
	if float64(total) >= float64(m.maxSize)*0.80 {
		target = int64(float64(m.maxSize) * 0.75)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].e.LastAccess.Before(items[j].e.LastAccess)
	})
	for _, it := range items {
		if total <= target {
			break
		}
		_ = m.deleteLocked(it.key)
		total -= it.e.Size
	}
	return nil
}

func fileSHA256Hex(path string) (string, error) {
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

