package npm

import (
	"fmt"

	npmhandler "github.com/livehl/mirrorhub/internal/handlers/npm"
	"github.com/livehl/mirrorhub/internal/platform"
)

const prefetchPriorityNPM = 60

func (p *NPMPlatform) PrefetchPriority() int { return prefetchPriorityNPM }

func (p *NPMPlatform) OwnsPrefetchItem(item string) bool {
	return npmhandler.IsTarballURL(item)
}

func (p *NPMPlatform) ExpandPrefetchItem(env platform.PrefetchExpandEnv, item string) ([]string, error) {
	pcfg, ok := env.Cfg.Platforms["npm"]
	if !ok || !pcfg.Enabled {
		return nil, fmt.Errorf("npm 模块未启用")
	}
	if !npmhandler.IsTarballURL(item) {
		return nil, fmt.Errorf("无效 npm tarball: %s", item)
	}
	return []string{item}, nil
}
