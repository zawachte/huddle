//go:build unix

package engine

import (
	"os"
	"syscall"
)

func statOwnership(info os.FileInfo) (int, int) {
	stat := info.Sys().(*syscall.Stat_t)
	return int(stat.Uid), int(stat.Gid)
}
