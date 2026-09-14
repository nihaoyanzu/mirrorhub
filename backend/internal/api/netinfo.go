package api

import (
	"net"
	"net/http"
	"sort"
	"strings"
)

type networkHint struct {
	URL   string `json:"url"`
	IP    string `json:"ip"`
	Iface string `json:"iface"`
}

func (s *Server) getNetworkHints(w http.ResponseWriter, _ *http.Request) {
	port := listenPort(s.cfg.Get().Server.ProxyAddr)
	hints := localIPv4Hints(port)
	writeJSON(w, http.StatusOK, map[string]any{
		"proxy_port":      port,
		"suggestions":     hints,
		"recommend_empty": true,
	})
}

// getPublicGuide 无需登录：返回已启用模块与本机下载地址，供说明页使用。
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
		"addresses":  localIPv4Hints(port),
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

func localIPv4Hints(port string) []networkHint {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []networkHint
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP == nil {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() {
				continue
			}
			ip := ip4.String()
			if _, dup := seen[ip]; dup {
				continue
			}
			seen[ip] = struct{}{}
			out = append(out, networkHint{
				URL:   "http://" + net.JoinHostPort(ip, port),
				IP:    ip,
				Iface: iface.Name,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Iface != out[j].Iface {
			return out[i].Iface < out[j].Iface
		}
		return out[i].IP < out[j].IP
	})
	return out
}
