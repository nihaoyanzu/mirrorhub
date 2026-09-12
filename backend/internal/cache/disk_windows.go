//go:build windows

package cache

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func diskFreeBytes(path string) (int64, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return 0, false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, false
	}
	vol := filepath.VolumeName(abs)
	if vol == "" {
		return 0, false
	}
	root := vol + `\`

	var freeAvail, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(windows.StringToUTF16Ptr(root), &freeAvail, &total, &totalFree); err != nil {
		return 0, false
	}
	return int64(freeAvail), true
}
