package traffic

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/livehl/mirrorhub/internal/store"
)

const (
	sampleInterval = time.Second
	rateWindow     = 5 * time.Second
	maxAccess      = 200
	maxSamples     = 30
	persistKey     = "stats.traffic"
	persistEvery   = 5 * time.Second
)

// Access 一次客户端资源请求记录
type Access struct {
	At       time.Time `json:"at"`
	IP       string    `json:"ip"`
	Method   string    `json:"method"`
	Path     string    `json:"path"`
	Platform string    `json:"platform"`
	Bytes    int64     `json:"bytes"`
	Cache    string    `json:"cache"`
	Status   string    `json:"status,omitempty"`
}

// Snapshot 供管理台展示（上游=从源站拉取，下游=向客户端分发）
type Snapshot struct {
	UpstreamBps     float64  `json:"upstream_bps"`
	DownstreamBps   float64  `json:"downstream_bps"`
	UpstreamTotal   int64    `json:"upstream_total"`
	DownstreamTotal int64    `json:"downstream_total"`
	WindowSeconds   float64  `json:"window_seconds"`
	Recent          []Access `json:"recent"`
	RecentCapacity  int      `json:"recent_capacity"` // 环形缓冲上限（按条数，非时间）
}

type sample struct {
	at   time.Time
	down uint64
	up   uint64
}

type persisted struct {
	DownloadTotal uint64   `json:"download_total"`
	UploadTotal   uint64   `json:"upload_total"`
	Recent        []Access `json:"recent"`
}

	// Recorder 统计上下行字节与近期访问（累计写入运营库）
	type Recorder struct {
	down atomic.Uint64
	up   atomic.Uint64

	mu      sync.Mutex
	samples []sample
	recent  []Access
	dirty   bool
	store   *store.Store
	stop    chan struct{}
	done    chan struct{}
}

func New(st *store.Store) *Recorder {
	r := &Recorder{
		samples: make([]sample, 0, maxSamples),
		recent:  make([]Access, 0, maxAccess),
		store:   st,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	r.load()
	go r.loop()
	return r
}

func (r *Recorder) load() {
	if r.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var p persisted
	ok, err := r.store.GetJSON(ctx, persistKey, &p)
	if err != nil || !ok {
		return
	}
	r.down.Store(p.DownloadTotal)
	r.up.Store(p.UploadTotal)
	if len(p.Recent) > 0 {
		r.mu.Lock()
		r.recent = append([]Access{}, p.Recent...)
		if len(r.recent) > maxAccess {
			r.recent = r.recent[len(r.recent)-maxAccess:]
		}
		r.mu.Unlock()
	}
}

func (r *Recorder) save() {
	if r.store == nil {
		return
	}
	r.mu.Lock()
	if !r.dirty {
		r.mu.Unlock()
		return
	}
	recent := append([]Access{}, r.recent...)
	r.dirty = false
	r.mu.Unlock()

	p := persisted{
		DownloadTotal: r.down.Load(),
		UploadTotal:   r.up.Load(),
		Recent:        recent,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = r.store.PutJSON(ctx, persistKey, p)
}

func (r *Recorder) loop() {
	defer close(r.done)
	sampleTick := time.NewTicker(sampleInterval)
	persistTick := time.NewTicker(persistEvery)
	defer sampleTick.Stop()
	defer persistTick.Stop()
	for {
		select {
		case <-r.stop:
			r.mu.Lock()
			r.dirty = true
			r.mu.Unlock()
			r.save()
			return
		case <-sampleTick.C:
			r.mu.Lock()
			r.samples = append(r.samples, sample{
				at:   time.Now(),
				down: r.down.Load(),
				up:   r.up.Load(),
			})
			if len(r.samples) > maxSamples {
				r.samples = r.samples[len(r.samples)-maxSamples:]
			}
			r.mu.Unlock()
		case <-persistTick.C:
			r.save()
		}
	}
}

func (r *Recorder) Close() {
	select {
	case <-r.stop:
	default:
		close(r.stop)
	}
	<-r.done
}

// AddDownload 累计上游拉取字节
func (r *Recorder) AddDownload(n int64) {
	if n > 0 {
		r.down.Add(uint64(n))
		r.mu.Lock()
		r.dirty = true
		r.mu.Unlock()
	}
}

// AddUpload 累计下游分发字节
func (r *Recorder) AddUpload(n int64) {
	if n > 0 {
		r.up.Add(uint64(n))
		r.mu.Lock()
		r.dirty = true
		r.mu.Unlock()
	}
}

func (r *Recorder) Record(a Access) {
	if a.At.IsZero() {
		a.At = time.Now()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recent = append(r.recent, a)
	if len(r.recent) > maxAccess {
		r.recent = r.recent[len(r.recent)-maxAccess:]
	}
	r.dirty = true
}

func (r *Recorder) Snapshot() Snapshot {
	downTotal := int64(r.down.Load())
	upTotal := int64(r.up.Load())

	r.mu.Lock()
	defer r.mu.Unlock()

	nowDown := r.down.Load()
	nowUp := r.up.Load()
	now := time.Now()

	var downBps, upBps, window float64
	if len(r.samples) > 0 {
		cutoff := now.Add(-rateWindow)
		base := r.samples[0]
		for _, s := range r.samples {
			if !s.at.Before(cutoff) {
				base = s
				break
			}
		}
		dt := now.Sub(base.at).Seconds()
		if dt > 0.2 {
			window = dt
			downBps = float64(nowDown-base.down) / dt
			upBps = float64(nowUp-base.up) / dt
		}
	}

	recent := make([]Access, len(r.recent))
	for i := range r.recent {
		recent[len(r.recent)-1-i] = r.recent[i]
	}

		return Snapshot{
			UpstreamBps:     downBps,
			DownstreamBps:   upBps,
			UpstreamTotal:   downTotal,
			DownstreamTotal: upTotal,
			WindowSeconds:   window,
			Recent:          recent,
			RecentCapacity:  maxAccess,
		}
}

// ClientIP 解析真实客户端 IP
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// CountWriter 统计写出字节并实时计入下游分发；尽量保留 Flusher
type CountWriter struct {
	http.ResponseWriter
	N   int64
	Rec *Recorder
}

func (w *CountWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if n > 0 {
		w.N += int64(n)
		if w.Rec != nil {
			w.Rec.AddUpload(int64(n))
		}
	}
	return n, err
}

func (w *CountWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
