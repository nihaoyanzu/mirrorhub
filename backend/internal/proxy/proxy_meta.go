package proxy

import (
	"net/http"
	"strconv"

	"github.com/livehl/mirrorhub/internal/cache"
	"github.com/livehl/mirrorhub/internal/config"
	"github.com/livehl/mirrorhub/internal/downloader"
	"github.com/livehl/mirrorhub/internal/platform"
	"github.com/livehl/mirrorhub/internal/router"
)

func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request, m *router.Match, cfg config.Config, pypi config.PlatformConfig, prefetch, boost bool, onAcquired func()) (string, error) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return s.handleIndexFallbackProxy(w, r, m, prefetch, boost, onAcquired)
	}
	if r.Method == http.MethodHead {
		return s.handleIndexFallbackProxy(w, r, m, prefetch, boost, onAcquired)
	}

	headers := platform.FilterRequestHeaders(r.Header)
	cacheKey := cache.KeyFromURL(m.TargetURL)
	opt := downloader.Options{
		URL:           m.TargetURL,
		SourceURL:     m.TargetURL,
		CacheKey:      cacheKey,
		TTLSeconds:    cfg.Cache.IndexTTLSeconds,
		Platform:      m.Platform,
		Prefetch:      prefetch,
		Boost:         boost,
		Headers:       headers,
		ChunkTTLHours: cfg.Cache.ChunkTTLHours,
		Kind:          "metadata",
		Concurrency:   pypi.Download.Concurrency,
		ChunkSize:     pypi.Download.ChunkSize,
		RangeHeader:   r.Header.Get("Range"),
		OnAcquired:    onAcquired,
	}
	if prefetch {
		res, err := s.dl.GetOrDownload(r.Context(), opt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return "miss", err
		}
		return serveDownloadResult(w, res, boost)
	}
	label, err := s.dl.ServePackage(r.Context(), w, opt)
	return label, err
}

// handleIndexFallbackProxy 非 GET 的简单透传（仍占任务槽+限速）
func (s *Server) handleIndexFallbackProxy(w http.ResponseWriter, r *http.Request, m *router.Match, prefetch, boost bool, onAcquired func()) (string, error) {
	release, err := s.acquireProxyTask(r.Context(), m.Platform, boost, onAcquired)
	if err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return "na", err
	}
	defer release()
	headers := platform.FilterRequestHeaders(r.Header)
	status, respHeader, body, err := s.dl.ProxyBytes(r.Context(), r.Method, m.TargetURL, headers, r.Body, m.Platform, prefetch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return "miss", err
	}
	ct := respHeader.Get("Content-Type")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	writeProxyHeaders(w, respHeader, ct, status, "MISS", "proxy", boost)
	_, _ = w.Write(body)
	return "miss", nil
}
