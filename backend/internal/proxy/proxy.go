package proxy

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/livehl/mirrorhub/internal/cache"
	"github.com/livehl/mirrorhub/internal/config"
	"github.com/livehl/mirrorhub/internal/downloader"
	pypihandler "github.com/livehl/mirrorhub/internal/handlers/pypi"
	"github.com/livehl/mirrorhub/internal/metrics"
	"github.com/livehl/mirrorhub/internal/platform"
	_ "github.com/livehl/mirrorhub/internal/platform/pypi" // 注册 PyPI 平台
	"github.com/livehl/mirrorhub/internal/ratelimit"
	"github.com/livehl/mirrorhub/internal/router"
	"github.com/livehl/mirrorhub/internal/scheduler"
	"github.com/livehl/mirrorhub/internal/traffic"
)

// revalResult 条件请求续期的结果，供并发请求共享
type revalResult struct {
	done         chan struct{}
	entry        *cache.Entry // 续期后的条目（nil 表示未续期成功）
	body         []byte       // 续期成功时的旧内容，或内容变更时的新正文
	contentType  string
	upstreamETag string // 内容变更路径写入缓存用
}

type Server struct {
	cfg      *config.Manager
	cache    *cache.Manager
	dl       *downloader.Engine
	sched    *scheduler.Scheduler
	limiters *ratelimit.Registry
	traffic  *traffic.Recorder
	log      *zap.Logger

	revalLocks sync.Map // cacheKey -> chan struct{}，per-key 续期锁防惊群
}

func New(cfg *config.Manager, c *cache.Manager, dl *downloader.Engine, sched *scheduler.Scheduler, lim *ratelimit.Registry, tr *traffic.Recorder, log *zap.Logger) *Server {
	return &Server{cfg: cfg, cache: c, dl: dl, sched: sched, limiters: lim, traffic: tr, log: log}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.HandleFunc("/*", s.handle)
	return r
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Get()
	mr := platform.Match(r.URL.Path, cfg)
	if mr == nil {
		http.Error(w, "no matching rule", http.StatusNotFound)
		return
	}
	m := mr.Match
	pcfg := cfg.Platforms[mr.Platform.Name()]

	prio := scheduler.PriorityInteractive
	if r.Header.Get("X-Mirrorhub-Priority") == "prefetch" {
		prio = scheduler.PriorityPrefetch
	}

	boost := false
	if prio != scheduler.PriorityPrefetch && cfg.Scheduler.SmallFileBoost.Enabled {
		if m.IsIndex || m.IsMetadata || m.SmallFileBoost {
			boost = true
		}
	}

	cw := &traffic.CountWriter{ResponseWriter: w, Rec: s.traffic}
	strategy := string(m.Strategy)
	taskID := s.sched.Begin(m.Platform, m.TargetURL, prio, boost)
	onAcquired := func() { s.sched.MarkRunning(taskID) }
	onProgress := func(done, total int64) { s.sched.UpdateProgress(taskID, done, total) }
	var taskErr error
	cacheLabel := "na"
	defer func() {
		// 极短命中路径可能未回调 OnAcquired，结束前兜底标 running 再 End
		s.sched.MarkRunning(taskID)
		s.sched.End(taskID, taskErr)
		metrics.RequestsTotal.WithLabelValues(m.Platform, strategy, cacheLabel).Inc()
		if s.traffic != nil {
			// 下游字节已在 CountWriter.Write 中实时计入，此处不再 AddUpload
			if prio != scheduler.PriorityPrefetch {
				status := "ok"
				if taskErr != nil {
					status = "error"
				}
				s.traffic.Record(traffic.Access{
					IP:       traffic.ClientIP(r),
					Method:   r.Method,
					Path:     r.URL.Path,
					Platform: m.Platform,
					Bytes:    cw.N,
					Cache:    cacheLabel,
					Status:   status,
				})
			}
		}
	}()

	switch {
	case m.IsIndex:
		strategy = "proxy"
		cacheLabel, taskErr = s.handleIndex(cw, r, m, mr.Platform, cfg, prio == scheduler.PriorityPrefetch, boost, onAcquired)
	case m.IsMetadata:
		strategy = "metadata"
		cacheLabel, taskErr = s.handleMetadata(cw, r, m, cfg, pcfg, prio == scheduler.PriorityPrefetch, boost, onAcquired)
	case m.Strategy == router.StrategyParallel:
		cacheLabel, strategy, taskErr = s.handlePackage(cw, r, m, cfg, pcfg, prio == scheduler.PriorityPrefetch, boost, onAcquired, onProgress)
	default:
		http.Error(cw, "unknown strategy", http.StatusInternalServerError)
	}
}

func (s *Server) acquireProxyTask(ctx context.Context, platform string, boost bool, onAcquired func()) (func(), error) {
	pl := s.limiters.Get(platform)
	if pl == nil {
		if onAcquired != nil {
			onAcquired()
		}
		return func() {}, nil
	}
	if err := pl.AcquireTask(ctx, boost); err != nil {
		return nil, err
	}
	if onAcquired != nil {
		onAcquired()
	}
	return func() { pl.ReleaseTask(boost) }, nil
}

func writeProxyHeaders(w http.ResponseWriter, respHeader http.Header, ct string, status int, cacheLabel, strategy string, boost bool) {
	outH := platform.FilterResponseHeaders(respHeader)
	for k, vals := range outH {
		lk := strings.ToLower(k)
		if lk == "content-type" {
			continue
		}
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("X-Cache", cacheLabel)
	w.Header().Set("X-Mirrorhub-Strategy", strategy)
	if boost {
		w.Header().Set("X-Mirrorhub-Boost", "1")
	}
	w.WriteHeader(status)
}
func rememberIndexDigests(body []byte, pageURL string) {
	for _, ref := range pypihandler.ExtractArtifactRefs(body, pageURL) {
		if ref.SHA256 != "" {
			downloader.RememberDigest(ref.URL, ref.SHA256)
		}
	}
}

func matchETag(inm, etag string) bool {
	inm = strings.TrimSpace(inm)
	if inm == "" || etag == "" {
		return false
	}
	if inm == "*" {
		return true
	}
	for _, part := range strings.Split(inm, ",") {
		if strings.TrimSpace(part) == etag {
			return true
		}
	}
	return false
}

func serveDownloadResult(w http.ResponseWriter, res *downloader.Result, boost bool) (string, error) {
	label, _, err := serveDownloadResultFull(w, res, boost)
	return label, err
}

func serveDownloadResultFull(w http.ResponseWriter, res *downloader.Result, boost bool) (string, bool, error) {
	f, err := os.Open(res.Entry.FilePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "miss", false, err
	}
	defer f.Close()
	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(res.Size, 10))
	cacheLabel := "miss"
	strat := res.Strategy
	if strat == "" {
		strat = "parallel"
	}
	if res.FromCache {
		w.Header().Set("X-Cache", "HIT")
		cacheLabel = "hit"
		strat = "cache"
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	w.Header().Set("X-Mirrorhub-Strategy", strat)
	if boost {
		w.Header().Set("X-Mirrorhub-Boost", "1")
	}
	_, err = io.Copy(w, f)
	return cacheLabel, false, err
}
