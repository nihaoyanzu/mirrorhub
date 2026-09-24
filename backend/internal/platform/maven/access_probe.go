package maven

import (
	"strings"

	"github.com/livehl/mirrorhub/internal/platform"
)

func (p *MavenPlatform) ProbeAccess(env platform.AccessProbeEnv) []platform.AccessProbeCheck {
	base := strings.TrimRight(strings.TrimSpace(env.Cfg.Upstream), "/")
	if base == "" {
		base = "https://repo1.maven.org/maven2"
	}
	// 下载极小 POM，验证仓库路径与制品拉取（仓库根常 404，不能当通过依据）
	pom := platform.JoinURL(base, "/junit/junit/4.13.2/junit-4.13.2.pom")
	return []platform.AccessProbeCheck{
		platform.ProbeDownloadSample(env.Ctx, env.Client, "download", pom, nil),
	}
}
