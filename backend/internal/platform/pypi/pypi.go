// Package pypi 实现 PyPI 平台的 Platform 接口。
package pypi

import (
	"bytes"
	"strings"

	"github.com/livehl/mirrorhub/internal/config"
	pypihandler "github.com/livehl/mirrorhub/internal/handlers/pypi"
	"github.com/livehl/mirrorhub/internal/platform"
	"github.com/livehl/mirrorhub/internal/router"
)

func init() {
	platform.Register(&PyPIPlatform{})
}

// PyPIPlatform 实现 platform.Platform 接口。
type PyPIPlatform struct{}

func (p *PyPIPlatform) Name() string { return "pypi" }

func (p *PyPIPlatform) Route(path string, cfg config.PlatformConfig) *router.Match {
	return MatchPyPI(path, Routes{
		Upstream:         cfg.Upstream,
		FileUpstream:     cfg.FileUpstream,
		MetadataUpstream: cfg.MetadataUpstream,
	})
}

type Routes struct {
	Upstream         string
	FileUpstream     string
	MetadataUpstream string
}

// MatchPyPI 将本代理路径映射到上游 TargetURL。
func MatchPyPI(path string, r Routes) *router.Match {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	switch {
	case strings.HasPrefix(path, "/simple"):
		return &router.Match{
			Platform:       "pypi",
			Strategy:       router.StrategyProxy,
			UpstreamBase:   r.Upstream,
			TargetURL:      strings.TrimRight(r.Upstream, "/") + path,
			IsIndex:        true,
			SmallFileBoost: true,
		}
	case strings.HasPrefix(path, "/packages") && strings.HasSuffix(path, ".metadata"):
		if !pypihandler.IsPackageArtifactPath(path) {
			return nil
		}
		metaUpstream := r.MetadataUpstream
		if metaUpstream == "" {
			metaUpstream = r.FileUpstream
		}
		return &router.Match{
			Platform:     "pypi",
			Strategy:     router.StrategyProxy,
			UpstreamBase: metaUpstream,
			TargetURL:    strings.TrimRight(metaUpstream, "/") + path,
			IsMetadata:   true,
		}
	case strings.HasPrefix(path, "/packages"):
		// 仅发行文件；裸 /packages 或目录页会命中上游 HTML 浏览页，一律不代理
		if !pypihandler.IsPackageArtifactPath(path) {
			return nil
		}
		return &router.Match{
			Platform:     "pypi",
			Strategy:     router.StrategyParallel,
			UpstreamBase: r.FileUpstream,
			TargetURL:    strings.TrimRight(r.FileUpstream, "/") + path,
		}
	default:
		return nil
	}
}

func (p *PyPIPlatform) TransformIndex(body []byte, contentType string, cfg config.Config, accept, pageURL string) platform.IndexResult {
	ct := platform.DetectContentType(contentType, body)
	body, ct = negotiateSimpleIndex(body, ct, accept, pageURL)
	proxyBase := pypihandler.PublicBaseURL(cfg)
	out := pypihandler.RewriteURLs(body, ct, cfg)
	etag := pypihandler.IndexETag(out, proxyBase)
	return platform.IndexResult{
		Body:        out,
		ETag:        etag,
		ContentType: ct,
	}
}

func negotiateSimpleIndex(body []byte, contentType, accept, pageURL string) ([]byte, string) {
	if !pypihandler.PrefersJSONAccept(accept) {
		return body, contentType
	}
	lower := strings.ToLower(contentType)
	trimmed := bytes.TrimSpace(body)
	if strings.Contains(lower, "json") || (len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')) {
		return body, "application/vnd.pypi.simple.v1+json"
	}
	out, err := pypihandler.HTMLSimpleToJSON(body, pageURL)
	if err != nil || len(out) == 0 {
		return body, contentType
	}
	return out, "application/vnd.pypi.simple.v1+json"
}
