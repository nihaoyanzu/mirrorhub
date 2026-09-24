package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/livehl/mirrorhub/internal/auth"
	"github.com/livehl/mirrorhub/internal/cache"
	"github.com/livehl/mirrorhub/internal/config"
	"github.com/livehl/mirrorhub/internal/downloader"
	npmhandler "github.com/livehl/mirrorhub/internal/handlers/npm"
	pypihandler "github.com/livehl/mirrorhub/internal/handlers/pypi"
	"github.com/livehl/mirrorhub/internal/metrics"
	"github.com/livehl/mirrorhub/internal/prefetch"
	"github.com/livehl/mirrorhub/internal/ratelimit"
	"github.com/livehl/mirrorhub/internal/scheduler"
	"github.com/livehl/mirrorhub/internal/traffic"
)

type Server struct {
	cfg      *config.Manager
	cache    *cache.Manager
	dl       *downloader.Engine
	limiters *ratelimit.Registry
	sched    *scheduler.Scheduler
	prefetch *prefetch.Service
	auth     *auth.Service
	traffic  *traffic.Recorder
	log      *zap.Logger
	webDir   string

	catalogMu         sync.RWMutex
	catalogNames      []string
	catalogNameSet    map[string]struct{}
	catalogURL        string
	catalogAt         time.Time
	catalogRefreshing bool
	catalogLastErr    string

	searchCacheMu sync.Mutex
	searchCache   map[string]searchCacheEntry
}

type searchCacheEntry struct {
	names []string
	at    time.Time
}

func New(cfg *config.Manager, c *cache.Manager, dl *downloader.Engine, lim *ratelimit.Registry, sched *scheduler.Scheduler, pf *prefetch.Service, authSvc *auth.Service, tr *traffic.Recorder, log *zap.Logger, webDir string) *Server {
	return &Server{
		cfg:         cfg,
		cache:       c,
		dl:          dl,
		limiters:    lim,
		sched:       sched,
		prefetch:    pf,
		auth:        authSvc,
		traffic:     tr,
		log:         log,
		webDir:      strings.TrimSpace(webDir),
		searchCache: map[string]searchCacheEntry{},
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.cors)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/login", s.login)
		r.Get("/public/guide", s.getPublicGuide)
		r.Group(func(r chi.Router) {
			r.Use(s.authRequired)
			r.Post("/logout", s.logout)
			r.Get("/me", s.me)
			r.Put("/password", s.changePassword)
			r.Get("/config", s.getConfig)
			r.Put("/config", s.putConfig)
			r.Post("/config/test", s.postAccessTest)
			r.Get("/stats", s.getStats)
			r.Get("/queue", s.getQueue)
			r.Delete("/queue", s.clearQueue)
			r.Post("/prefetch", s.postPrefetch)
			r.Delete("/prefetch", s.deletePrefetch)
			r.Get("/cache", s.getCache)
			r.Delete("/cache", s.clearCache)
			r.Get("/packages", s.listPackages)
			r.Get("/packages/{name}", s.getPackage)
			r.Delete("/packages/entry", s.deletePackageEntry)
		})
	})

	if s.webDir != "" {
		r.NotFound(spaFileServer(s.webDir))
	}
	return r
}

// spaFileServer 托管前端构建产物；找不到文件时回退 index.html（Vue Router history）
func spaFileServer(webDir string) http.HandlerFunc {
	root := http.Dir(webDir)
	fs := http.FileServer(root)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := root.Open(path); err == nil {
				_ = f.Close()
				fs.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	}
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func (s *Server) authRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := s.auth.Validate(bearerToken(r)); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body loginReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token, err := s.auth.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			http.Error(w, "用户名或密码错误", http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"username": body.Username,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.auth.Logout(bearerToken(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, err := s.auth.Validate(bearerToken(r))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"username": user})
}

type changePasswordReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	user, err := s.auth.Validate(bearerToken(r))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body changePasswordReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
		if err := s.auth.ChangePassword(r.Context(), user, body.OldPassword, body.NewPassword); err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				// 用 400 而非 401，避免前端把「旧密码错误」当成会话失效并跳转登录
				http.Error(w, "当前密码不正确", http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) getConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Get())
}

type configUpdate struct {
	UpstreamProxy *string                          `json:"upstream_proxy"`
	Cache         *config.CacheConfig              `json:"cache"`
	RateLimit     *config.RateLimitConfig          `json:"rate_limit"`
	Scheduler     *config.SchedulerConfig          `json:"scheduler"`
	Platforms     map[string]config.PlatformConfig `json:"platforms"`
}

func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	var body configUpdate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	prevUpstream := s.catalogRootURL()
	err := s.cfg.Update(func(c *config.Config) {
		if body.UpstreamProxy != nil {
			c.Server.UpstreamProxy = *body.UpstreamProxy
		}
		if body.Cache != nil {
			c.Cache.MaxSizeGB = body.Cache.MaxSizeGB
			c.Cache.IndexTTLSeconds = body.Cache.IndexTTLSeconds
			c.Cache.PackageTTLSeconds = body.Cache.PackageTTLSeconds
			c.Cache.ChunkTTLHours = body.Cache.ChunkTTLHours
		}
		if body.RateLimit != nil {
			c.RateLimit = *body.RateLimit
		}
		if body.Scheduler != nil {
			c.Scheduler = *body.Scheduler
		}
		if body.Platforms != nil {
			if c.Platforms == nil {
				c.Platforms = map[string]config.PlatformConfig{}
			}
			for name, p := range body.Platforms {
				p.RateLimit = nil
				c.Platforms[name] = p
			}
		}
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg := s.cfg.Get()
	if body.RateLimit != nil {
		s.limiters.Update(cfg.RateLimit)
	}
	s.limiters.SetIdleRatio(cfg.Scheduler.Prefetch.IdleQuotaRatio)
	if body.Cache != nil {
		s.cache.SetMaxSizeGB(cfg.Cache.MaxSizeGB)
	}
	if body.UpstreamProxy != nil && s.dl != nil {
		s.dl.SetUpstreamProxy(cfg.Server.UpstreamProxy)
	}
	if s.catalogRootURL() != prevUpstream {
		s.kickCatalogRefresh(true)
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) getStats(w http.ResponseWriter, _ *http.Request) {
	cs := s.cache.Stats()
	metrics.CacheUsageRatio.Set(cs.UsageRatio)
	metrics.DiskFreeBytes.Set(float64(cs.DiskFreeBytes))
	metrics.SchedulerP0Active.Set(float64(s.sched.InteractiveActive()))
	out := map[string]any{
		"cache":              cs,
		"rate_limiters":      s.limiters.Snapshots(),
		"interactive_active": s.sched.InteractiveActive(),
		"resume_count":       s.sched.ResumeCount(),
	}
	if s.traffic != nil {
		out["traffic"] = s.traffic.Snapshot()
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getQueue(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"tasks": s.sched.List()})
}

func (s *Server) clearQueue(w http.ResponseWriter, _ *http.Request) {
	n := s.sched.ClearFinished()
	writeJSON(w, http.StatusOK, map[string]any{"cleared": n})
}

type prefetchReq struct {
	URLs []string `json:"urls"`
	Text string   `json:"text"` // 粘贴的 requirements / 依赖文件内容
}

func (s *Server) postPrefetch(w http.ResponseWriter, r *http.Request) {
	var body prefetchReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	items := append([]string{}, body.URLs...)
	var skipped []string
	if strings.TrimSpace(body.Text) != "" {
		if npmhandler.LookLikeLockfile(body.Text) {
			parsed, skip := npmhandler.ParseLockfile(body.Text)
			items = append(items, parsed...)
			skipped = skip
		} else {
			parsed, skip := pypihandler.ParseDependencyText(body.Text)
			items = append(items, parsed...)
			skipped = skip
		}
	}
	// 去重保序
	seen := map[string]struct{}{}
	uniq := make([]string, 0, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		if _, ok := seen[it]; ok {
			continue
		}
		seen[it] = struct{}{}
		uniq = append(uniq, it)
	}
	if len(uniq) == 0 {
		msg := "未解析到可预拉取的包需求"
		if len(skipped) > 0 {
			msg += "（已跳过选项/VCS 等行）"
		}
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	s.prefetch.Enqueue(uniq)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"enqueued": len(uniq),
		"items":    uniq,
		"skipped":  skipped,
	})
}

func (s *Server) deletePrefetch(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	ok := s.prefetch.Cancel(id)
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": ok})
}

func (s *Server) getCache(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.cache.Stats())
}

func (s *Server) clearCache(w http.ResponseWriter, _ *http.Request) {
	if err := s.cache.Clear(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cleared": true})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
