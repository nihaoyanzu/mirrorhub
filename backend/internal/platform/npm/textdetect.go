package npm

import (
	npmhandler "github.com/livehl/mirrorhub/internal/handlers/npm"
	"github.com/livehl/mirrorhub/internal/platform"
)

const textDetectPriorityNPM = 100

func (p *NPMPlatform) TextDetectPriority() int { return textDetectPriorityNPM }

func (p *NPMPlatform) LookLikePrefetchText(text string) bool {
	return npmhandler.LookLikeLockfile(text)
}

func (p *NPMPlatform) ParsePrefetchText(text string) (platform.TextDetectResult, bool) {
	if !npmhandler.LookLikeLockfile(text) {
		return platform.TextDetectResult{}, false
	}
	parsed, skip := npmhandler.ParseLockfile(text)
	return platform.TextDetectResult{Items: parsed, Skipped: skip, Kind: "npm_lock"}, true
}
