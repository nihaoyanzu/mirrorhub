package npm

import (
	npmhandler "github.com/livehl/mirrorhub/internal/handlers/npm"
	"github.com/livehl/mirrorhub/internal/platform"
)

const prefetchPriorityNPM = 60

func (p *NPMPlatform) PrefetchPriority() int { return prefetchPriorityNPM }

func (p *NPMPlatform) OwnsPrefetchItem(item string) bool {
	return npmhandler.IsTarballURL(item)
}

func (p *NPMPlatform) ExpandPrefetchItem(_ platform.PrefetchExpandEnv, item string) ([]string, error) {
	return []string{item}, nil
}
