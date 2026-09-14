package proxy

import (
	"net/http"
	"strings"

	"github.com/livehl/mirrorhub/internal/config"
)

// applyRequestPublicHost：PublicHost 为空或为本机回环时，用客户端实际访问的 Host 改写索引链接。
// 监听地址本身已是全局的；PublicHost 只影响索引里的下载 URL，不能靠「监听」替代。
func applyRequestPublicHost(cfg config.Config, r *http.Request) config.Config {
	reqHost := strings.TrimSpace(r.Host)
	if reqHost == "" {
		return cfg
	}
	ph := strings.TrimSpace(cfg.Server.PublicHost)
	if ph != "" && !isLoopbackPublicHost(ph) {
		return cfg
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if xp := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); xp != "" {
		scheme = strings.ToLower(strings.TrimSpace(strings.Split(xp, ",")[0]))
		if scheme != "http" && scheme != "https" {
			scheme = "http"
		}
	}
	cfg.Server.PublicHost = scheme + "://" + reqHost
	return cfg
}

func isLoopbackPublicHost(ph string) bool {
	h := strings.ToLower(strings.TrimSpace(ph))
	h = strings.TrimPrefix(h, "http://")
	h = strings.TrimPrefix(h, "https://")
	if i := strings.IndexByte(h, '/'); i >= 0 {
		h = h[:i]
	}
	host := h
	if i := strings.IndexByte(h, ':'); i >= 0 {
		host = h[:i]
	}
	return host == "127.0.0.1" || host == "localhost" || host == "::1" || host == "[::1]"
}
