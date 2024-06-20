/*
Copyright 2024 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package probes

import (
	"os"
	"path"

	"golang.org/x/sys/unix"
)

func (a *assertFileSystem) Probe(reporter Reporter) error {
	var stat unix.Statfs_t
	for p := a.fsPath; ; {
		if err := unix.Statfs(p, &stat); err != nil {
			if os.IsNotExist(err) {
				if parent := path.Dir(p); parent != p {
					p = parent
					continue
				}
			}
			return reporter.Error(a.desc(), err)
		}
		a.fsPath = p
		break
	}

	var fs string
	// https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/plain/include/uapi/linux/magic.h
	// we need to type cast to uint to avoid compile error on arm 32bit:
	//    unix.BPF_FS_MAGIC (untyped int constant 3405662737) overflows int32
	// See: https://github.com/golang/go/issues/44794
	switch uint(stat.Type) {
	case unix.AAFS_MAGIC:
		fs = "aafs"
	case unix.ADFS_SUPER_MAGIC:
		fs = "adfs"
	case unix.AFFS_SUPER_MAGIC:
		fs = "affs"
	case unix.AFS_FS_MAGIC:
		fs = "afs_fs"
	case unix.AFS_SUPER_MAGIC:
		fs = "afs"
	case unix.ANON_INODE_FS_MAGIC:
		fs = "anon_inode_fs"
	case unix.AUTOFS_SUPER_MAGIC:
		fs = "autofs"
	case unix.BDEVFS_MAGIC:
		fs = "bdevfs"
	case unix.BINDERFS_SUPER_MAGIC:
		fs = "binderfs"
	case unix.BINFMTFS_MAGIC:
		fs = "binfmtfs"
	case unix.BPF_FS_MAGIC:
		fs = "bpf_fs"
	case unix.BTRFS_SUPER_MAGIC:
		fs = "btrfs"
	case unix.BTRFS_TEST_MAGIC:
		fs = "btrfs_test"
	case unix.CEPH_SUPER_MAGIC:
		fs = "ceph"
	case unix.CGROUP2_SUPER_MAGIC:
		fs = "cgroup2"
	case unix.CGROUP_SUPER_MAGIC:
		fs = "cgroup"
	case unix.CIFS_SUPER_MAGIC:
		fs = "cifs"
	case unix.CODA_SUPER_MAGIC:
		fs = "coda"
	case unix.CRAMFS_MAGIC:
		fs = "cramfs"
	case unix.DAXFS_MAGIC:
		fs = "daxfs"
	case unix.DEBUGFS_MAGIC:
		fs = "debugfs"
	case unix.DEVMEM_MAGIC:
		fs = "devmem"
	case unix.DEVPTS_SUPER_MAGIC:
		fs = "devpts"
	case unix.DMA_BUF_MAGIC:
		fs = "dma_buf"
	case unix.ECRYPTFS_SUPER_MAGIC:
		fs = "ecryptfs"
	case unix.EFIVARFS_MAGIC:
		fs = "efivarfs"
	case unix.EFS_SUPER_MAGIC:
		fs = "efs"
	case unix.EROFS_SUPER_MAGIC_V1:
		fs = "erofs v1"
	case unix.EXFAT_SUPER_MAGIC:
		fs = "exfat"
	case unix.EXT4_SUPER_MAGIC:
		fs = "ext4"
	case unix.F2FS_SUPER_MAGIC:
		fs = "f2fs"
	case unix.FUSE_SUPER_MAGIC:
		fs = "fuse"
	case unix.FUTEXFS_SUPER_MAGIC:
		fs = "futexfs"
	case unix.HOSTFS_SUPER_MAGIC:
		fs = "hostfs"
	case unix.HPFS_SUPER_MAGIC:
		fs = "hpfs"
	case unix.HUGETLBFS_MAGIC:
		fs = "hugetlbfs"
	case unix.ISOFS_SUPER_MAGIC:
		fs = "isofs"
	case unix.JFFS2_SUPER_MAGIC:
		fs = "jffs2"
	case unix.MINIX2_SUPER_MAGIC:
		fs = "minix v2"
	case unix.MINIX2_SUPER_MAGIC2:
		fs = "minix v2 (30 char names)"
	case unix.MINIX3_SUPER_MAGIC:
		fs = "minix v3"
	case unix.MINIX_SUPER_MAGIC:
		fs = "minix"
	case unix.MINIX_SUPER_MAGIC2:
		fs = "minix (30 char names)"
	case unix.MSDOS_SUPER_MAGIC:
		fs = "msdos"
	case unix.MTD_INODE_FS_MAGIC:
		fs = "mtd_inode_fs"
	case unix.NCP_SUPER_MAGIC:
		fs = "ncp"
	case unix.NFS_SUPER_MAGIC:
		fs = "nfs"
	case unix.NILFS_SUPER_MAGIC:
		fs = "nilfs"
	case unix.NSFS_MAGIC:
		fs = "nsfs"
	case unix.OCFS2_SUPER_MAGIC:
		fs = "ocfs2"
	case unix.OPENPROM_SUPER_MAGIC:
		fs = "openprom"
	case unix.OVERLAYFS_SUPER_MAGIC:
		fs = "overlayfs"
	case unix.PID_FS_MAGIC:
		fs = "pid_fs"
	case unix.PIPEFS_MAGIC:
		fs = "pipefs"
	case unix.PROC_SUPER_MAGIC:
		fs = "proc"
	case unix.PSTOREFS_MAGIC:
		fs = "pstorefs"
	case unix.QNX4_SUPER_MAGIC:
		fs = "qnx4"
	case unix.QNX6_SUPER_MAGIC:
		fs = "qnx6"
	case unix.RAMFS_MAGIC:
		fs = "ramfs"
	case unix.RDTGROUP_SUPER_MAGIC:
		fs = "rdtgroup"
	case unix.REISERFS_SUPER_MAGIC:
		fs = "reiserfs"
	case unix.SECRETMEM_MAGIC:
		fs = "secretmem"
	case unix.SECURITYFS_MAGIC:
		fs = "securityfs"
	case unix.SELINUX_MAGIC:
		fs = "selinux"
	case unix.SMACK_MAGIC:
		fs = "smack"
	case unix.SMB2_SUPER_MAGIC:
		fs = "smb2"
	case unix.SMB_SUPER_MAGIC:
		fs = "smb"
	case unix.SOCKFS_MAGIC:
		fs = "sockfs"
	case unix.SQUASHFS_MAGIC:
		fs = "squashfs"
	case unix.STACK_END_MAGIC:
		fs = "stack_end"
	case unix.SYSFS_MAGIC:
		fs = "sysfs"
	case unix.TMPFS_MAGIC:
		fs = "tmpfs"
	case unix.TRACEFS_MAGIC:
		fs = "tracefs"
	case unix.UDF_SUPER_MAGIC:
		fs = "udf"
	case unix.USBDEVICE_SUPER_MAGIC:
		fs = "usbdevice"
	case unix.V9FS_MAGIC:
		fs = "v9fs"
	case unix.XENFS_SUPER_MAGIC:
		fs = "xenfs"
	case unix.XFS_SUPER_MAGIC:
		fs = "xfs"
	case unix.ZONEFS_MAGIC:
		fs = "zonefs"
	default:
		return reporter.Warn(a.desc(), StringProp("unknown"), "")
	}

	return reporter.Pass(a.desc(), StringProp(fs))
}
