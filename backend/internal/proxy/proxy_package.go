package proxy

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/livehl/mirrorhub/internal/cache"
	"github.com/livehl/mirrorhub/internal/config"
	"github.com/livehl/mirrorhub/internal/downloader"
	pypihandler "github.com/livehl/mirrorhub/internal/handlers/pypi"
	"github.com/livehl/mirrorhub/internal/platform"
	"github.com/livehl/mirrorhub/internal/router"
)

func (s *Server) handlePackage(w http.ResponseWriter, r *http.Request, m *router.Match, cfg config.Config, pypi config.PlatformConfig, prefetch, boost bool, onAcquired func(), onProgress func(done, total int64)) (string, string, error) {
	origURL := m.TargetURL
	headers := platform.FilterRequestHeaders(r.Header)
	cacheKey := cache.KeyFromURL(origURL)

	if entry, ok := s.cache.Get(cacheKey); ok {
		if onAcquired != nil {
			onAcquired()
		}
		if onProgress != nil {
			onProgress(entry.Size, entry.Size)
		}
		etag := entry.ETag
		if etag == "" && entry.Digest != "" {
			etag = `"` + entry.Digest + `"`
		}
		if etag == "" {
			etag = fmt.Sprintf(`W/"%d"`, entry.Size)
		}
		if r.Method == http.MethodHead || r.Method == http.MethodGet {
			if matchETag(r.Header.Get("If-None-Match"), etag) {
				w.Header().Set("ETag", etag)
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Length", strconv.FormatInt(entry.Size, 10))
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(http.StatusNotModified)
				return "hit", "cache", nil
			}
		}
		if r.Method == http.MethodHead {
			ct := entry.ContentType
			if ct == "" {
				ct = "application/octet-stream"
			}
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Content-Length", strconv.FormatInt(entry.Size, 10))
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("ETag", etag)
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			return "hit", "cache", nil
		}
		// GET 命中：直接回本地缓存，禁止再打上游 HEAD（否则每次 HIT 都被上游探测拖慢）
		if r.Method == http.MethodGet {
			label, err := s.dl.ServeCachedEntry(w, entry, boost, r.Header.Get("Range"))
			return label, "cache", err
		}
	}

	// HEAD 请求直接探测上游文件大小，绕过 task semaphore 避免被暂停的 prefetch 任务阻塞。
	if r.Method == http.MethodHead {
		// 给 HEAD 请求一个短超时，避免被上游慢连接阻塞
		headCtx, headCancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer headCancel()
		size, ct, _, headErr := s.dl.Head(headCtx, origURL, headers)
		if headErr != nil {
			http.Error(w, headErr.Error(), http.StatusBadGateway)
			return "miss", "proxy", headErr
		}
		if pypihandler.IsHTMLContentType(ct) {
			err := fmt.Errorf("upstream returned HTML for package path (not an artifact)")
			http.Error(w, err.Error(), http.StatusBadGateway)
			return "miss", "proxy", err
		}
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("X-Cache", "MISS")
		w.Header().Set("X-Mirrorhub-Strategy", "proxy")
		if boost {
			w.Header().Set("X-Mirrorhub-Boost", "1")
		}
		w.WriteHeader(http.StatusOK)
		return "miss", "proxy", nil
	}

	if r.Method != http.MethodGet {
		c, err := s.handleIndexFallbackProxy(w, r, m, prefetch, boost, onAcquired)
		return c, "proxy", err
	}

	fetchURL := origURL
	var size int64 = -1
	hasSize := false
	hs, headCT, finalURL, headErr := s.dl.Head(r.Context(), origURL, headers)
	if headErr == nil {
		if pypihandler.IsHTMLContentType(headCT) {
			err := fmt.Errorf("upstream returned HTML for package path (not an artifact)")
			http.Error(w, err.Error(), http.StatusBadGateway)
			return "miss", "proxy", err
		}
		size = hs
		hasSize = true
		if finalURL != "" {
			fetchURL = finalURL
		}
	}

	fileBoost := boost
	if !prefetch && cfg.Scheduler.SmallFileBoost.Enabled {
		maxBoost := int64(cfg.Scheduler.SmallFileBoost.MaxSizeKB) * 1024
		if hasSize && size >= 0 {
			if (pypi.Download.MinSize > 0 && size < pypi.Download.MinSize) || (maxBoost > 0 && size <= maxBoost) {
				fileBoost = true
			}
		}
	}

	opt := downloader.Options{
		URL:            fetchURL,
		SourceURL:      origURL,
		CacheKey:       cacheKey,
		Concurrency:    pypi.Download.Concurrency,
		ChunkSize:      pypi.Download.ChunkSize,
		MinSize:        pypi.Download.MinSize,
		TTLSeconds:     cfg.Cache.PackageTTLSeconds,
		Platform:       m.Platform,
		Prefetch:       prefetch,
		Boost:          fileBoost,
		Headers:        headers,
		ChunkTTLHours:  cfg.Cache.ChunkTTLHours,
		Kind:           "package",
		HasKnownSize:   hasSize,
		KnownSize:      size,
		ExpectedSHA256: downloader.LookupDigest(origURL),
		RangeHeader:    r.Header.Get("Range"),
		OnAcquired:     onAcquired,
		OnProgress:     onProgress,
	}

	if !prefetch {
		label, err := s.dl.ServePackage(r.Context(), w, opt)
		strat := "stream"
		if label == "hit" {
			strat = "cache"
		} else if hasSize && size >= pypi.Download.MinSize && pypi.Download.MinSize > 0 {
			strat = "parallel-stream"
		}
		return label, strat, err
	}

	if err := s.cache.AllowPrefetchWrite(); err != nil {
		http.Error(w, err.Error(), http.StatusInsufficientStorage)
		return "miss", "parallel", err
	}

	res, err := s.dl.GetOrDownload(r.Context(), opt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return "miss", "parallel", err
	}
	label, _, err := serveDownloadResultFull(w, res, fileBoost)
	strat := res.Strategy
	if strat == "" {
		strat = "parallel"
	}
	return label, strat, err
}
