//go:build !windows

package api

import "syscall"

// diskSpace returns the space left to this process and the size of the
// filesystem that holds dir. ok is false when the filesystem cannot be read.
func diskSpace(dir string) (free, total int64, ok bool) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(dir, &fs); err != nil {
		return 0, 0, false
	}
	return int64(fs.Bavail) * int64(fs.Bsize), int64(fs.Blocks) * int64(fs.Bsize), true
}
