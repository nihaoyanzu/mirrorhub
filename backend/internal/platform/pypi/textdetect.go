package pypi

import (
	"strings"

	pypihandler "github.com/livehl/mirrorhub/internal/handlers/pypi"
	"github.com/livehl/mirrorhub/internal/platform"
)

const textDetectPriorityPyPI = 0

func (p *PyPIPlatform) TextDetectPriority() int { return textDetectPriorityPyPI }

func (p *PyPIPlatform) LookLikePrefetchText(text string) bool {
	return strings.TrimSpace(text) != ""
}

func (p *PyPIPlatform) ParsePrefetchText(text string) (platform.TextDetectResult, bool) {
	parsed, skip := pypihandler.ParseDependencyText(text)
	return platform.TextDetectResult{Items: parsed, Skipped: skip, Kind: "pypi_req"}, true
}
