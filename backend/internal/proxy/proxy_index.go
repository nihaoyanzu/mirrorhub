package proxy

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/livehl/mirrorhub/internal/cache"
	"github.com/livehl/mirrorhub/internal/config"
	"github.com/livehl/mirrorhub/internal/platform"
	"github.com/livehl/mirrorhub/internal/router"
)

// handleIndex 缓存改写前正文，每次响应按客户端访问 Host 改写
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request, m *router.Match, plat platform.Platform, cfg config.Config, prefetch, boost bool, onAcquired func()) (string, error) {
	cfg = applyRequestPublicHost(cfg, r)
	release, err := s.acquireProxyTask(r.Context(), m.Platform, boost, onAcquired)
	if err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return "na", err
	}
	defer release()

	cacheKey := m.Platform + ":index:" + cache.KeyFromURL(m.TargetURL)

	// ---------- 1. 缓存命中（未过期）→ 直接返回 ----------
	if r.Method == http.MethodGet {
		if entry, ok := s.cache.Get(cacheKey); ok {
			return s.serveCachedIndex(w, r, entry, plat, cfg, boost)
		}
	}

	// ---------- 2. 缓存 miss：尝试条件请求续期（per-key 去重防惊群）----------
	headers := platform.FilterRequestHeaders(r.Header)
	// 客户端条件请求针对我们改写后的 ETag，不能原样转给上游
	headers.Del("If-None-Match")
	headers.Del("If-Modified-Since")

	if r.Method == http.MethodGet {
		if stale, ok := s.cache.GetStale(cacheKey); ok {
			result := s.doRevalidation(cacheKey, m.TargetURL, m.Platform, stale, headers, cfg.Cache.IndexTTLSeconds)
			<-result.done
			if result.entry != nil && len(result.body) > 0 {
				// 续期成功 → 用旧内容响应
				return s.serveRevalidated(w, r, result, plat, cfg, boost)
			}
			// 续期拿到了新正文：写入缓存后按 MISS 响应，避免再打一次上游
			if len(result.body) > 0 && result.contentType != "" {
				body := result.body
				body, _ = platform.MaybeGunzip(body)
				ct := platform.DetectContentType(result.contentType, body)
				if _, err := s.cache.PutBytes(cacheKey, body, ct, cfg.Cache.IndexTTLSeconds, cache.Meta{
					SourceURL:    m.TargetURL,
					Kind:         "index",
					UpstreamETag: result.upstreamETag,
				}); err != nil && s.log != nil {
					s.log.Warn("index cache put failed", zap.String("key", cacheKey), zap.Error(err))
				}
				rememberIndexDigests(body, m.TargetURL)
				tr := plat.TransformIndex(body, ct, cfg, r.Header.Get("Accept"), m.TargetURL)
				w.Header().Set("Content-Length", strconv.Itoa(len(tr.Body)))
				w.Header().Set("ETag", tr.ETag)
				writeProxyHeaders(w, http.Header{}, tr.ContentType, http.StatusOK, "MISS", "index", boost)
				_, _ = w.Write(tr.Body)
				return "miss", nil
			}
			// 续期失败 → 回退到全量 GET（下面的逻辑）
		}
	}

	// ---------- 3. 全量 GET ----------
	status, respHeader, body, getErr := s.dl.ProxyBytes(r.Context(), r.Method, m.TargetURL, headers, r.Body, m.Platform, prefetch)
	if getErr != nil {
		http.Error(w, getErr.Error(), http.StatusBadGateway)
		return "miss", getErr
	}

	body, _ = platform.MaybeGunzip(body)
	ct := platform.DetectContentType(respHeader.Get("Content-Type"), body)
	if r.Method == http.MethodGet && status >= 200 && status < 300 {
		if _, err := s.cache.PutBytes(cacheKey, body, ct, cfg.Cache.IndexTTLSeconds, cache.Meta{
			SourceURL:    m.TargetURL,
			Kind:         "index",
			UpstreamETag: respHeader.Get("ETag"),
		}); err != nil && s.log != nil {
			s.log.Warn("index cache put failed", zap.String("key", cacheKey), zap.Error(err))
		}
	}
	rememberIndexDigests(body, m.TargetURL)
	result := plat.TransformIndex(body, ct, cfg, r.Header.Get("Accept"), m.TargetURL)
	if matchETag(r.Header.Get("If-None-Match"), result.ETag) {
		w.Header().Set("ETag", result.ETag)
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(http.StatusNotModified)
		return "miss", nil
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(result.Body)))
	w.Header().Set("ETag", result.ETag)
	writeProxyHeaders(w, respHeader, result.ContentType, status, "MISS", "index", boost)
	_, _ = w.Write(result.Body)
	return "miss", nil
}

// doRevalidation 执行 per-key 条件请求续期，防止惊群效应。
// 优先 If-None-Match；镜像无 ETag/不支持 304 时回退全文比对。
func (s *Server) doRevalidation(cacheKey, targetURL, platName string, stale *cache.Entry, headers http.Header, ttlSeconds int) *revalResult {
	val, loaded := s.revalLocks.LoadOrStore(cacheKey, &revalResult{done: make(chan struct{})})
	result := val.(*revalResult)
	if loaded {
		return result
	}
	go func() {
		defer func() {
			s.revalLocks.Delete(cacheKey)
			close(result.done)
		}()

		oldData, readErr := os.ReadFile(stale.FilePath)
		if readErr != nil {
			if s.log != nil {
				s.log.Warn("revalidation read stale file failed",
					zap.String("path", stale.FilePath), zap.Error(readErr))
			}
			return
		}

		renewOK := func() bool {
			if err := s.cache.RenewTTL(cacheKey, ttlSeconds); err != nil {
				if s.log != nil {
					s.log.Warn("RenewTTL failed", zap.String("key", cacheKey), zap.Error(err))
				}
				return false
			}
			result.entry = stale
			result.body = oldData
			result.contentType = stale.ContentType
			return true
		}

		if strings.TrimSpace(stale.UpstreamETag) != "" {
			condHeaders := headers.Clone()
			condHeaders.Set("If-None-Match", stale.UpstreamETag)
			condStatus, condErr := s.dl.ProxyHEAD(context.Background(), targetURL, condHeaders)
			if condErr == nil && condStatus == http.StatusNotModified {
				_ = renewOK()
				return
			}
			if condErr != nil && s.log != nil {
				s.log.Warn("revalidation HEAD failed, will fallback to GET",
					zap.String("url", targetURL), zap.Error(condErr))
			} else if condStatus >= 400 && s.log != nil {
				s.log.Warn("revalidation upstream error",
					zap.String("url", targetURL), zap.Int("status", condStatus))
			}
		}

		// 无 ETag 或不支持 304：全量 GET 后比对正文
		getHeaders := headers.Clone()
		getHeaders.Del("If-None-Match")
		getHeaders.Del("If-Modified-Since")
		status, respHeader, body, getErr := s.dl.ProxyBytes(context.Background(), http.MethodGet, targetURL, getHeaders, nil, platName, false)
		if getErr != nil {
			if s.log != nil {
				s.log.Warn("revalidation GET failed", zap.String("url", targetURL), zap.Error(getErr))
			}
			return
		}
		if status < 200 || status >= 300 {
			return
		}
		body, _ = platform.MaybeGunzip(body)
		oldNorm, _ := platform.MaybeGunzip(oldData)
		if bytes.Equal(body, oldNorm) {
			_ = renewOK()
			return
		}
		// 内容已变：交给调用方写入新缓存，避免再 GET 一次
		result.body = body
		result.contentType = platform.DetectContentType(respHeader.Get("Content-Type"), body)
		result.upstreamETag = respHeader.Get("ETag")
	}()
	return result
}

// serveRevalidated 从续期结果响应客户端
func (s *Server) serveRevalidated(w http.ResponseWriter, r *http.Request, result *revalResult, plat platform.Platform, cfg config.Config, boost bool) (string, error) {
	data, _ := platform.MaybeGunzip(result.body)
	ct := platform.DetectContentType(result.contentType, data)
	rememberIndexDigests(data, result.entry.SourceURL)
	tr := plat.TransformIndex(data, ct, cfg, r.Header.Get("Accept"), result.entry.SourceURL)
	if matchETag(r.Header.Get("If-None-Match"), tr.ETag) {
		w.Header().Set("ETag", tr.ETag)
		w.Header().Set("X-Cache", "REVALIDATED")
		w.WriteHeader(http.StatusNotModified)
		return "hit", nil
	}
	w.Header().Set("Content-Type", tr.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(tr.Body)))
	w.Header().Set("ETag", tr.ETag)
	w.Header().Set("X-Cache", "REVALIDATED")
	w.Header().Set("X-Mirrorhub-Strategy", "index")
	if boost {
		w.Header().Set("X-Mirrorhub-Boost", "1")
	}
	_, _ = w.Write(tr.Body)
	return "hit", nil
}

// serveCachedIndex 从缓存响应索引请求（ETag/304、URL 改写）
func (s *Server) serveCachedIndex(w http.ResponseWriter, r *http.Request, entry *cache.Entry, plat platform.Platform, cfg config.Config, boost bool) (string, error) {
	data, err := os.ReadFile(entry.FilePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "na", err
	}
	data, _ = platform.MaybeGunzip(data)
	ct := platform.DetectContentType(entry.ContentType, data)
	rememberIndexDigests(data, entry.SourceURL)
	tr := plat.TransformIndex(data, ct, cfg, r.Header.Get("Accept"), entry.SourceURL)
	if matchETag(r.Header.Get("If-None-Match"), tr.ETag) {
		w.Header().Set("ETag", tr.ETag)
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusNotModified)
		return "hit", nil
	}
	w.Header().Set("Content-Type", tr.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(tr.Body)))
	w.Header().Set("ETag", tr.ETag)
	w.Header().Set("X-Cache", "HIT")
	w.Header().Set("X-Mirrorhub-Strategy", "index")
	if boost {
		w.Header().Set("X-Mirrorhub-Boost", "1")
	}
	_, _ = w.Write(tr.Body)
	return "hit", nil
}
