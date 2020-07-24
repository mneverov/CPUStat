package main

import (
	"path/filepath"

	"golang.org/x/sys/unix"
)

const unifiedMountpoint = "/sys/fs/cgroup"

// from https://github.com/containerd/cgroups/blob/master/utils.go#L61
func GetCGroupMode() (cgMode string) {
	var st unix.Statfs_t
	if err := unix.Statfs(unifiedMountpoint, &st); err != nil {
		cgMode = "Unavailable"
	}
	switch st.Type {
	case unix.CGROUP2_SUPER_MAGIC:
		return "Unified"
	default:
		cgMode = "Legacy"
		if err := unix.Statfs(filepath.Join(unifiedMountpoint, "unified"), &st); err != nil {
			return
		}
		if st.Type == unix.CGROUP2_SUPER_MAGIC {
			cgMode = "Hybrid"
		}
	}
	return
}
