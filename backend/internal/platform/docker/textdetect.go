package docker

import (
	"strings"

	dockerhandler "github.com/livehl/mirrorhub/internal/handlers/docker"
	"github.com/livehl/mirrorhub/internal/platform"
)

const textDetectPriorityDocker = 60

func (p *DockerPlatform) TextDetectPriority() int { return textDetectPriorityDocker }

func (p *DockerPlatform) LookLikePrefetchText(text string) bool {
	return dockerhandler.LookLikeImageList(text)
}

func (p *DockerPlatform) ParsePrefetchText(text string) (platform.TextDetectResult, bool) {
	if !dockerhandler.LookLikeImageList(text) {
		return platform.TextDetectResult{}, false
	}
	refs, skip := dockerhandler.ParseImageList(text)
	items := make([]string, 0, len(refs))
	for _, ref := range refs {
		if strings.HasPrefix(ref.Tag, "sha256:") {
			items = append(items, ref.Repo+"@"+ref.Tag)
		} else {
			items = append(items, ref.Repo+":"+ref.Tag)
		}
	}
	return platform.TextDetectResult{Items: items, Skipped: skip, Kind: "docker_image"}, true
}
