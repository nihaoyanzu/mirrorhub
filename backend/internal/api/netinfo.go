package api

import (
	"net"
	"net/http"
	"sort"
	"strings"
)

// getPublicGuide 无需登录：返回已启用模块与下载端口，供说明页按访问 Host 拼地址。
func (s *Server) getPublicGuide(w http.ResponseWriter, _ *http.Request) {
	cfg := s.cfg.Get()
	port := listenPort(cfg.Server.ProxyAddr)
	names := make([]string, 0, len(cfg.Platforms))
	for name := range cfg.Platforms {
		names = append(names, name)
	}
	sort.Strings(names)
	modules := make([]map[string]any, 0, len(names))
	for _, name := range names {
		p := cfg.Platforms[name]
		modules = append(modules, map[string]any{
			"id":      name,
			"enabled": p.Enabled,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"proxy_port": port,
		"modules":    modules,
	})
}

func listenPort(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "8081"
	}
	if _, port, err := net.SplitHostPort(addr); err == nil && port != "" {
		return port
	}
	if strings.HasPrefix(addr, ":") {
		p := strings.TrimPrefix(addr, ":")
		if p != "" {
			return p
		}
	}
	return "8081"
}
