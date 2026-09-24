package huggingface

import (
	"context"
	"net/http"
	"strings"

	"github.com/livehl/mirrorhub/internal/config"
	hfhandler "github.com/livehl/mirrorhub/internal/handlers/huggingface"
)

func (p *HuggingFacePlatform) InjectUpstreamAuth(_ context.Context, headers http.Header, cfg config.Config, _ string) error {
	pcfg := cfg.Platforms["huggingface"]
	tok := strings.TrimSpace(pcfg.UpstreamToken)
	if tok == "" {
		return nil
	}
	headers.Set("Authorization", "Bearer "+tok)
	return nil
}

func (p *HuggingFacePlatform) PackageCacheIdentity(_, targetURL string, reqHeader http.Header, defaultKey string) (string, string) {
	key := "hf:pkg:" + defaultKey
	// defaultKey 是 KeyFromURL；调用方传入的 defaultKey 已是 KeyFromURL(origURL)
	// 这里约定：调用方传 cache.KeyFromURL(origURL) 作为 defaultKey，我们包一层前缀。
	_ = targetURL
	if etag := reqHeader.Get("X-Linked-Etag"); etag != "" {
		if bk := hfhandler.BlobCacheKeyFromETag(etag); bk != "" {
			return bk, ""
		}
	}
	return key, ""
}

func (p *HuggingFacePlatform) DetectIndexContentType(upstreamCT, _ string, body []byte) string {
	return hfhandler.DetectContentType("", upstreamCT, body)
}
