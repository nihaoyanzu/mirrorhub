// Package cachekey 提供与缓存存储无关的键派生，供 handlers / cache / proxy 共用，避免 handlers→cache 环依赖。
package cachekey

import (
	"crypto/sha256"
	"encoding/hex"
)

// FromURL 由 URL 派生稳定短键（与历史 cache.KeyFromURL 算法一致）。
func FromURL(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:16])
}
