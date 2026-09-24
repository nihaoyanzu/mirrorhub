package huggingface

import (
	"net/http"
	"strings"

	hfhandler "github.com/livehl/mirrorhub/internal/handlers/huggingface"
	"github.com/livehl/mirrorhub/internal/platform"
)

func (p *HuggingFacePlatform) ProbeAccess(env platform.AccessProbeEnv) []platform.AccessProbeCheck {
	base := strings.TrimRight(strings.TrimSpace(env.Cfg.Upstream), "/")
	if base == "" {
		base = "https://huggingface.co"
	}
	// 公开小文件：gpt2/config.json（数 KB）
	fileURL := hfhandler.ResolveFileURL(base, "model", "gpt2", "main", "config.json")
	var hdr http.Header
	if tok := strings.TrimSpace(env.Cfg.UpstreamToken); tok != "" {
		hdr = http.Header{}
		hdr.Set("Authorization", "Bearer "+tok)
	}
	return []platform.AccessProbeCheck{
		platform.ProbeDownloadSample(env.Ctx, env.Client, "download", fileURL, hdr),
	}
}
