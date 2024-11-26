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

package cleanup

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	osunix "github.com/k0sproject/k0s/internal/os/unix"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type directories struct {
	Config *Config
}

// Name returns the name of the step
func (d *directories) Name() string {
	return "remove directories step"
}

func (d *directories) Run() error {
	log := logrus.StandardLogger()

	var errs []error
	if err := cleanupBeneath(log, d.Config.dataDir); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete data directory: %w", err))
	}
	if err := cleanupBeneath(log, d.Config.runDir); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete run directory: %w", err))
	}
	return errors.Join(errs...)
}

const (
	cleanupOFlags       = unix.O_NOFOLLOW
	cleanupAtFlags      = unix.AT_NO_AUTOMOUNT | unix.AT_SYMLINK_NOFOLLOW
	cleanupResolveFlags = unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS
)

// Recursively removes the specified directory. Attempts to do this by making
// sure that everything not in that directory is left untouched, i.e. the
// recursion will not follow any file system links such as symlinks and mount
// points. Instead, any mount points will be unmounted recursively.
//
// Note that this code assumes to be run with elevated privileges.
func cleanupBeneath(log logrus.FieldLogger, dirPath string) (err error) {
	// The real path is required as the code may be checking the mount info via
	// the proc filesystem.
	realDirPath, err := filepath.EvalSymlinks(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	dir, err := osunix.OpenDir(realDirPath, cleanupOFlags)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, dir.Close()) }()

	empty, err := cleanupPathNames(log, dir, realDirPath, true)
	if err != nil {
		return err
	}
	if empty {
		if err := os.Remove(realDirPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.WithError(err).Warn("Leaving behind ", realDirPath)
		}
	}

	return nil
}

func cleanupPathNames(log logrus.FieldLogger, dir *osunix.DirFD, dirPath string, unlink bool) (bool, error) {
	var leftovers bool
	for name, err := range dir.ReadEntryNames() {
		if err != nil {
			return false, fmt.Errorf("failed to enumerate directory entries: %w", err)
		}
		if !cleanupPathNameLoop(log, dir, dirPath, name, unlink) {
			leftovers = true
		}
	}

	return !leftovers, nil
}

type cleanupOutcome uint8

const (
	cleanIgnored cleanupOutcome = iota + 1
	cleanRetry
	cleanUnlinked
)

func cleanupPathNameLoop(log logrus.FieldLogger, dir *osunix.DirFD, dirPath, name string, unlink bool) bool {
	for attempt := 1; ; attempt++ {
		outcome, err := cleanupPathName(log, dir, dirPath, name, unlink)
		if err == nil {
			switch outcome {
			case cleanUnlinked:
				return true
			case cleanIgnored:
				return false
			case cleanRetry:
				if attempt < 256 {
					log.Debugf("Retrying %s/%s after attempt %d (unlink=%t)", dirPath, name, attempt, unlink)
					continue
				}
				err = errors.New("too many attempts")
			default:
				log.WithError(err).Errorf("Unexpected outcome while cleaning up %s/%s: %d", dirPath, name, outcome)
				return false
			}
		}

		if errors.Is(err, os.ErrNotExist) {
			return true
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}

		log.WithError(err).Warnf("Leaving behind %s/%s", dirPath, name)
		return false
	}
}

func cleanupPathName(log logrus.FieldLogger, dir *osunix.DirFD, dirPath, name string, unlink bool) (_ cleanupOutcome, err error) {
	if unlink {
		log.Debugf("Trying to unlink %s/%s", dirPath, name)
		if outcome, err := unlinkPathName(log, dir, dirPath, name); err != nil {
			return 0, err
		} else if outcome == unlinkUnlinked {
			return cleanUnlinked, nil
		} else if outcome == unlinkUnmounted {
			// Path has been unmounted. Retry to catch overmounts.
			return cleanRetry, nil
		}
	}

	// Try to recurse into the directory.
	log.Debugf("Trying to to open %s/%s", dirPath, name)
	subDir, isMountPoint, err := openDirName(dir, dirPath, name)
	if err != nil {
		// When not unlinking and this is not a directory,
		// it might be a mounted file. Try to unmount it.
		if !unlink && errors.Is(err, unix.ENOTDIR) {
			status, err := getPathNameMountStatus(dir, dirPath, name)
			if err != nil {
				return 0, err
			}

			if status == pathMountStatusRegular {
				// Definitely not a mount point. Ignore the file.
				return cleanIgnored, nil
			}

			err = unmount(log, filepath.Join(dirPath, name))
			if err == nil {
				// Path has been unmounted. Retry to catch overmounts.
				return cleanRetry, nil
			}
			if status == pathMountStatusUnknown && errors.Is(err, unix.EINVAL) {
				// Not a mount point (or the mount point is locked).
				return cleanIgnored, nil
			}
			return 0, err
		}

		return 0, err
	}

	close := true
	defer func() {
		if close {
			err = errors.Join(err, subDir.Close())
		}
	}()

	// Disable recursive unlink if it's a mount point.
	if isMountPoint {
		unlink = false
	}

	var empty bool
	subDirPath := filepath.Join(dirPath, name)
	empty, err = cleanupPathNames(log, subDir, subDirPath, unlink)
	if err != nil {
		return 0, err
	}

	// The subDir can be closed now. In fact, it must be closed, so that a
	// potential unmount will work.
	close = false
	if err := subDir.Close(); err != nil {
		return 0, err
	}

	if isMountPoint {
		if err := unmount(log, subDirPath); err != nil {
			return 0, err
		}
		return cleanRetry, nil
	}

	if unlink && empty {
		if err := dir.RemoveDir(name); err != nil {
			return 0, err
		}

		return cleanUnlinked, nil
	}

	return cleanIgnored, nil
}

type unlinkOutcome uint8

const (
	unlinkUnlinked unlinkOutcome = iota + 1
	unlinkRecurse
	unlinkUnmounted
)

func unlinkPathName(log logrus.FieldLogger, dir *osunix.DirFD, dirPath, name string) (unlinkOutcome, error) {
	// First try to simply unlink the name.
	// The assumption here is that mount points cannot be simply unlinked.
	fileErr := dir.Remove(name)
	if fileErr == nil || errors.Is(fileErr, os.ErrNotExist) {
		// That worked. Mission accomplished.
		return unlinkUnlinked, nil
	}

	// Try to remove an empty directory.
	dirErr := dir.RemoveDir(name)
	switch {
	case dirErr == nil:
		// That worked. Mission accomplished.
		return unlinkUnlinked, nil

	case errors.Is(dirErr, os.ErrExist):
		// It's a non-empty directory.
		return unlinkRecurse, nil

	case errors.Is(dirErr, unix.ENOTDIR):
		// It's not a directory. If it's a mount point, try to unmount it.
		if status, err := getPathNameMountStatus(dir, dirPath, name); err != nil {
			return 0, errors.Join(fileErr, err)
		} else if status != pathMountStatusRegular {
			if err := unmount(log, filepath.Join(dirPath, name)); err != nil {
				return 0, errors.Join(fileErr, err)
			}
			return unlinkUnmounted, nil
		}
		return 0, fileErr

	default:
		// Try to clean up recursively for all other errors.
		return unlinkRecurse, nil
	}
}

func openDirName(dir *osunix.DirFD, dirPath, name string) (_ *osunix.DirFD, isMountPoint bool, _ error) {
	// Try to use openat2 to open it in a way that won't cross mounts.
	subDir, err := dir.OpenDir2(name, unix.OpenHow{
		Flags:   cleanupOFlags,
		Resolve: cleanupResolveFlags | unix.RESOLVE_NO_XDEV,
	})

	// Did we try to cross a mount point?
	if errors.Is(err, unix.EXDEV) {
		isMountPoint = true
		subDir, err = dir.OpenDir2(name, unix.OpenHow{
			Flags:   cleanupOFlags,
			Resolve: cleanupResolveFlags,
		})
	}

	if err == nil || !errors.Is(err, errors.ErrUnsupported) {
		return subDir, isMountPoint, err
	}

	// Fallback to legacy open.
	subDir, err = dir.OpenDir(name, cleanupOFlags)
	if err != nil {
		return nil, false, err
	}

	close := true
	defer func() {
		if close {
			err = errors.Join(err, subDir.Close())
		}
	}()

	subDirPath := filepath.Join(dirPath, name)
	status, err := getPathMountStatus(dir, subDir, subDirPath)
	if err != nil {
		return nil, false, err
	}
	if status == pathMountStatusMountPoint {
		isMountPoint = true
	} else if status == pathMountStatusUnknown {
		// There's still no bullet-proof evidence to rule out that path is
		// actually a mount point. As a last resort, have a look at the proc fs.
		isMountPoint, err = mountInfoListsMountPoint("/proc/self/mountinfo", subDirPath)
		if err != nil {
			// The proc filesystem check failed, too. No other checks are left.
			// Assume that it's not a mount point.
			isMountPoint = false
		}
	}

	close = false
	return subDir, isMountPoint, nil
}

type pathMountStatus uint8

const (
	pathMountStatusUnknown pathMountStatus = iota
	pathMountStatusRegular
	pathMountStatusMountPoint
)

func getPathNameMountStatus(dir *osunix.DirFD, dirPath, name string) (pathMountStatus, error) {
	if path, err := dir.Open2(name, unix.OpenHow{
		Flags:   cleanupOFlags | unix.O_PATH,
		Resolve: cleanupResolveFlags | unix.RESOLVE_NO_XDEV,
	}); err == nil {
		return pathMountStatusRegular, path.Close()
	} else if errors.Is(err, unix.EXDEV) {
		return pathMountStatusMountPoint, nil
	} else if !errors.Is(err, errors.ErrUnsupported) {
		return 0, err
	}

	path, err := dir.Open(name, cleanupOFlags|unix.O_PATH, 0)
	if err != nil {
		return 0, err
	}

	defer func() { err = errors.Join(err, path.Close()) }()
	return getPathMountStatus(dir, path, filepath.Join(dirPath, name))
}

func getPathMountStatus(dir *osunix.DirFD, fd osunix.LinuxPath, path string) (pathMountStatus, error) {
	// Don't bother to try statx() here. The interesting fields (stx_mnt_id) and
	// attributes (STATX_ATTR_MOUNT_ROOT) have been introduced in Linux 5.8,
	// whereas openat2() is a thing since Linux 5.6. So its highly unlikely that
	// those will be available when openat2() isn't.

	// Check if the paths have different device numbers.
	if dirStat, err := dir.StatSelf(); err != nil {
		return 0, err
	} else if pathStat, err := fd.StatSelf(); err != nil {
		return 0, err
	} else if dirStat.Dev != pathStat.Dev {
		return pathMountStatusMountPoint, nil
	}

	// Try to expire the mount point.
	err := unix.Unmount(path, unix.MNT_EXPIRE|unix.UMOUNT_NOFOLLOW)
	switch {
	case errors.Is(err, unix.EINVAL):
		// This is the expected error when path is not a mount point. Note that
		// there's still the chance that path is referring to a locked mount
		// point, i.e. a mount point that is part of a more privileged mount
		// namespace than k0s is in. That's not easy to rule out ...
		// See https://www.man7.org/linux/man-pages/man2/umount.2.html#ERRORS.
		// See https://man7.org/linux/man-pages/man7/mount_namespaces.7.html.
		return pathMountStatusUnknown, nil

	case errors.Is(err, unix.EBUSY):
		// This is the expected error when path is a mount point. It indicates
		// that the resource is in use, which is guaranteed because there's an
		// open file descriptor for it.
		return pathMountStatusMountPoint, nil

	case errors.Is(err, unix.EAGAIN):
		// This is the expected error when path is an unused mount point. This
		// shouldn't happen, since there's still an open file descriptor to path.
		return 0, &os.PathError{
			Op:   "unmount",
			Path: path,
			Err:  fmt.Errorf("supposedly unreachable code path: %w", err),
		}

	case errors.Is(err, unix.EPERM):
		// This is the expected error if k0s doesn't have the privileges to
		// unmount path. Since this code should be run with root privileges,
		// this is not expected to happen. Anyhow, don't bail out.
		return pathMountStatusUnknown, nil

	case err == nil:
		// This means that the path was unmounted, as it has already been
		// expired before. This shouldn't happen, since there's still an open
		// file descriptor to path.
		return 0, &os.PathError{
			Op:   "unmount",
			Path: path,
			Err:  errors.New("supposedly unreachable code path: success"),
		}

	default:
		// Pass on all other errors.
		return 0, &os.PathError{Op: "unmount", Path: path, Err: err}
	}
}

// Checks whether path is listed as a mount point in the proc filesystems
// mountinfo file.
//
// https://man7.org/linux/man-pages/man5/proc_pid_mountinfo.5.html
func mountInfoListsMountPoint(mountInfoPath, path string) (bool, error) {
	mountInfoBytes, err := os.ReadFile(mountInfoPath)
	if err != nil {
		return false, err
	}

	mountInfoScanner := bufio.NewScanner(bytes.NewReader(mountInfoBytes))
	for mountInfoScanner.Scan() {
		// The fifth field is the mount point.
		fields := bytes.SplitN(mountInfoScanner.Bytes(), []byte{' '}, 6)
		// Some characters are octal-escaped, most notably the space character.
		if len(fields) > 5 && equalsOctalsUnsecaped(fields[4], path) {
			return true, nil
		}
	}

	return false, mountInfoScanner.Err()
}

// Compares if data and str are equal, converting any octal escape sequences of
// the form \NNN in data to their respective ASCII character on the fly.
func equalsOctalsUnsecaped(data []byte, str string) bool {
	dlen, slen := len(data), len(str)

	// An escape sequence takes 4 bytes.
	// The unescaped length of data is in range [dlen/4, dlen].
	if slen < dlen/4 || slen > dlen {
		return false // Lengths don't match, data and str cannot be equal.
	}

	doff := 0
	for soff := 0; soff < slen; soff, doff = soff+1, doff+1 {
		if doff >= dlen {
			return false // str is longer than unescaped data
		}
		ch := data[doff]
		if ch == '\\' && doff < dlen-3 { // The next three bytes should be octal digits.
			d1, d2, d3 := data[doff+1]-'0', data[doff+2]-'0', data[doff+3]-'0'
			// The ASCII character range is [0, 127] decimal, which corresponds
			// to [0, 177] octal. Check if the digits are in range.
			if d1 <= 1 && d2 <= 7 && d3 <= 7 {
				ch = d1<<6 | d2<<3 | d3 // Convert from octal digits (3 bits per digit).
				doff += 3               // Skip the three digits in the next iteration.
			}
		}

		if str[soff] != ch {
			return false
		}
	}

	return doff == dlen // Both are equal if data has been fully read.
}

func unmount(log logrus.FieldLogger, path string) error {
	log.Debug("Attempting to unmount ", path)
	if err := unix.Unmount(path, unix.UMOUNT_NOFOLLOW); err != nil {
		return &os.PathError{Op: "unmount", Path: path, Err: err}
	}

	log.Info("Unmounted ", path)
	return nil
}
