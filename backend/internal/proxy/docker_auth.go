package proxy

import (
	"net/http"
	"strings"
	"sync"

	"github.com/livehl/mirrorhub/internal/config"
	dockerhandler "github.com/livehl/mirrorhub/internal/handlers/docker"
)

// dockerAuth 懒加载按配置重建的 TokenSource。
type dockerAuth struct {
	mu     sync.Mutex
	src    *dockerhandler.TokenSource
	auth   string
	svc    string
	client *http.Client
}

func (s *Server) dockerTokens(cfg config.Config) *dockerhandler.TokenSource {
	pcfg := cfg.Platforms["docker"]
	authBase := strings.TrimSpace(pcfg.MetadataUpstream)
	if authBase == "" {
		authBase = "https://auth.docker.io"
	}
	svc := dockerhandler.AuthServiceFromRegistry(pcfg.Upstream)
	if s.dockerAuth == nil {
		s.dockerAuth = &dockerAuth{}
	}
	s.dockerAuth.mu.Lock()
	defer s.dockerAuth.mu.Unlock()
	if s.dockerAuth.src != nil && s.dockerAuth.auth == authBase && s.dockerAuth.svc == svc {
		return s.dockerAuth.src
	}
	s.dockerAuth.src = dockerhandler.NewTokenSource(authBase, svc, nil)
	s.dockerAuth.auth = authBase
	s.dockerAuth.svc = svc
	return s.dockerAuth.src
}

// injectDockerAuth 为回源请求注入 Bearer；repo 从 path 解析。
func (s *Server) injectDockerAuth(r *http.Request, headers http.Header, cfg config.Config, path string) error {
	repo := ""
	if rp, _, ok := dockerhandler.ParseManifestPath(path); ok {
		repo = rp
	} else if rp, _, ok := dockerhandler.ParseBlobPath(path); ok {
		repo = rp
	} else {
		return nil
	}
	tok, err := s.dockerTokens(cfg).Bearer(r.Context(), repo)
	if err != nil {
		return err
	}
	headers.Set("Authorization", "Bearer "+tok)
	return nil
}

func (s *Server) invalidateDockerAuth(cfg config.Config, path string) {
	repo := ""
	if rp, _, ok := dockerhandler.ParseManifestPath(path); ok {
		repo = rp
	} else if rp, _, ok := dockerhandler.ParseBlobPath(path); ok {
		repo = rp
	}
	if repo == "" {
		return
	}
	s.dockerTokens(cfg).Invalidate(repo)
}
