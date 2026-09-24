package proxy

import (
	"net/http"
	"strings"

	"github.com/livehl/mirrorhub/internal/config"
	hfhandler "github.com/livehl/mirrorhub/internal/handlers/huggingface"
)

// injectHFAuth 若配置了 UpstreamToken，回源注入 Bearer（覆盖客户端 Authorization）。
func injectHFAuth(headers http.Header, pcfg config.PlatformConfig) {
	tok := strings.TrimSpace(pcfg.UpstreamToken)
	if tok == "" {
		return
	}
	headers.Set("Authorization", "Bearer "+tok)
}

func stripXetFromHeader(h http.Header) {
	if h == nil {
		return
	}
	hfhandler.StripXetHeaders(h)
}
