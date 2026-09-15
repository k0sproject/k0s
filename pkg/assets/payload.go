// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package assets

import (
	"archive/zip"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"time"
)

// ErrNoPayload indicates that an executable has no ZIP payload appended to it.
// This is the case for k0s executables built with EMBEDDED_BINS_BUILDMODE=none.
var ErrNoPayload = errors.New("no payload attached")

// Payload provides read-only access to the ZIP archive that is appended to a
// k0s executable.
//
// Payload implements [fs.FS]. Entry names are slash-separated and don't have a
// leading slash. Embedded executables live in [EmbeddedBinDir]; the archive's
// root is reserved. Directories may be opened and read, even if the archive
// doesn't contain any explicit entries for them.
//
// Payloads must be closed after use. Files obtained from Open must be closed
// before closing the payload itself.
type Payload struct {
	zip     *zip.ReadCloser
	modTime time.Time
}

var _ fs.FS = (*Payload)(nil)

// OpenSelfPayload opens the ZIP payload that is appended to the currently
// running executable. Returns ErrNoPayload if there's no ZIP archive
// appended to it.
func OpenSelfPayload() (*Payload, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("unable to determine current executable: %w", err)
	}

	return openPayload(path)
}

// openPayload opens the ZIP payload that is appended to the executable at path.
// Returns ErrNoPayload if there's no ZIP archive appended to it.
func openPayload(path string) (*Payload, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%w: %s", syscall.EISDIR, path)
	}

	// Note that OpenReader may return a usable reader alongside an error, e.g.
	// for ErrInsecurePath, in which case closing it is up to the caller.
	zipFile, err := zip.OpenReader(path)
	if err != nil {
		if zipFile != nil {
			err = errors.Join(err, zipFile.Close())
		}

		// An invalid ZIP file means that this is a bare executable, without any
		// ZIP payload appended to it.
		if errors.Is(err, zip.ErrFormat) {
			return nil, ErrNoPayload
		}

		return nil, fmt.Errorf("while opening the payload of %s: %w", path, err)
	}

	return &Payload{zipFile, info.ModTime()}, nil
}

// Open opens the named payload entry, using the semantics of [fs.FS.Open].
func (p *Payload) Open(name string) (fs.File, error) {
	return p.zip.Open(name)
}

// ModTime returns the modification time of the executable that this payload is
// appended to. The modification times recorded inside the archive itself are not
// meaningful, since the k0s build doesn't set them.
func (p *Payload) ModTime() time.Time {
	return p.modTime
}

// Close closes the payload.
func (p *Payload) Close() error {
	return p.zip.Close()
}

// ContentID returns an opaque identifier for the contents of the payload entry
// described by info, which is expected to have been obtained from a [Payload].
// The identifier changes whenever the entry's contents change, and is stable
// across rebuilds, copies and reinstallations of the executable. Returns false
// if info doesn't describe a payload entry whose contents can be identified.
//
// Note that the identifier is not a cryptographic checksum. It is only intended
// to detect changed contents, not to protect against tampering.
func ContentID(info fs.FileInfo) (string, bool) {
	header, ok := info.Sys().(*zip.FileHeader)
	if !ok || info.IsDir() {
		return "", false
	}

	// Empty entries have no CRC32 checksum recorded, but there's nothing to
	// distinguish them by, either.
	if header.CRC32 == 0 && header.UncompressedSize64 == 0 {
		return "", false
	}

	return fmt.Sprintf("crc32:%08x/%d", header.CRC32, header.UncompressedSize64), true
}
