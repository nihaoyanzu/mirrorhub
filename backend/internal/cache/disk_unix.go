//go:build unix

package cache

import (
	"path/filepath"
	"strings"
	"syscall"
)

func diskFreeBytes(path string) (int64, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return 0, false
	}
	for p := path; ; p = filepath.Dir(p) {
		var st syscall.Statfs_t
		if err := syscall.Statfs(p, &st); err == nil {
			bsize := st.Bsize
			if st.Frsize > 0 {
				bsize = st.Frsize
			}
			if bsize <= 0 {
				return 0, false
			}
			return int64(st.Bavail) * int64(bsize), true
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
	}
	return 0, false
}
