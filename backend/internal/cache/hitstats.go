package cache

import "strings"

// hitCounters 全局与分平台命中计数；平台标签由调用方传入，本结构不解析平台语义。
type hitCounters struct {
	hits             int64
	misses           int64
	hitsByPlatform   map[string]int64
	missesByPlatform map[string]int64
}

func (h *hitCounters) noteHit(platform string) {
	h.hits++
	h.bump(&h.hitsByPlatform, platform)
}

func (h *hitCounters) noteMiss(platform string) {
	h.misses++
	h.bump(&h.missesByPlatform, platform)
}

func (h *hitCounters) bump(dst *map[string]int64, platform string) {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		platform = "other"
	}
	if *dst == nil {
		*dst = make(map[string]int64)
	}
	(*dst)[platform]++
}

func (h *hitCounters) clear() {
	h.hits = 0
	h.misses = 0
	h.hitsByPlatform = nil
	h.missesByPlatform = nil
}

func (h *hitCounters) load(hits, misses int64, byHit, byMiss map[string]int64) {
	h.hits = hits
	h.misses = misses
	h.hitsByPlatform = cloneInt64Map(byHit)
	h.missesByPlatform = cloneInt64Map(byMiss)
}

func (h *hitCounters) snapshot() (hits, misses int64, byHit, byMiss map[string]int64) {
	return h.hits, h.misses, cloneInt64Map(h.hitsByPlatform), cloneInt64Map(h.missesByPlatform)
}

func cloneInt64Map(m map[string]int64) map[string]int64 {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
