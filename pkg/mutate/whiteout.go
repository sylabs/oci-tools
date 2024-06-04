// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const (
	aufsWhiteoutPrefix = ".wh."
	aufsOpaqueMarker   = ".wh..wh..opq"
	schilyOpaqueXattr  = "SCHILY.xattr.trusted.overlay.opaque"
)

var errUnexpectedOpaque = errors.New("unexpected opaque marker")

// scanAUFSWhiteouts reads a TAR stream, returning a map of <path>:true for
// directories in the tar that contain an AUFS .wh..wh..opq opaque directory
// marker file, and a boolean indicating the presence of any .wh.<file> markers.
func scanAUFSWhiteouts(in io.Reader) (map[string]bool, bool, error) {
	opaquePaths := map[string]bool{}
	fileWhiteout := false

	tr := tar.NewReader(in)
	for {
		header, err := tr.Next()

		if err == io.EOF {
			return opaquePaths, fileWhiteout, nil
		}
		if err != nil {
			return nil, false, err
		}

		parent, base := filepath.Split(header.Name)

		if base == aufsOpaqueMarker {
			opaquePaths[parent] = true
		}

		if !fileWhiteout && strings.HasPrefix(base, aufsWhiteoutPrefix) {
			fileWhiteout = true
		}
	}
}

// whiteoutsToOverlayFS streams a tar file from in to out, replacing AUFS
// whiteout markers with OverlayFS whiteout markers. Due to unrestricted
// ordering of markers vs their target, the list of opaquePaths must be obtained
// prior to filtering and provided to this filter.
func whiteoutsToOverlayFS(in io.Reader, out io.Writer, opaquePaths map[string]bool) error {
	tr := tar.NewReader(in)
	tw := tar.NewWriter(out)
	defer tw.Close()

	for {
		header, err := tr.Next()

		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Must force to PAX format, to accommodate xattrs
		header.Format = tar.FormatPAX

		parent, base := filepath.Split(header.Name)
		// Don't include .wh..wh..opq opaque directory markers in output.
		if base == aufsOpaqueMarker {
			// If we don't know the target should be opaque, then provided opaquePaths is incorrect.
			if !opaquePaths[parent] {
				return fmt.Errorf("%q: %w", parent, errUnexpectedOpaque)
			}
			continue
		}
		// Set overlayfs xattr on a dir that was previously found to contain a .wh..wh..opq marker.
		if opq := opaquePaths[header.Name]; opq {
			if header.PAXRecords == nil {
				header.PAXRecords = map[string]string{}
			}
			header.PAXRecords[schilyOpaqueXattr] = "y"
		}
		// Replace a `.wh.<name>` marker with a char dev 0 at <name>
		if strings.HasPrefix(base, aufsWhiteoutPrefix) {
			target := parent + strings.TrimPrefix(base, aufsWhiteoutPrefix)
			header.Name = target
			header.Typeflag = tar.TypeChar
			header.Devmajor = 0
			header.Devminor = 0
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			continue
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// Disable gosec G110: Potential DoS vulnerability via decompression bomb.
		// We are just filtering a flow directly from tar reader to tar writer - we aren't reading
		// into memory beyond the stdlib buffering.
		//nolint:gosec
		if _, err := io.Copy(tw, tr); err != nil {
			return err
		}
	}
}

// whiteoutsToAUFS streams a tar file from in to out, replacing OverlayFS
// whiteout markers with AUFS whiteout markers.
func whiteoutsToAUFS(in io.Reader, out io.Writer) error {
	tr := tar.NewReader(in)
	tw := tar.NewWriter(out)
	defer tw.Close()

	for {
		header, err := tr.Next()

		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// <dir> with opaque xattr -> write both <dir> & <dir>/.wh..wh..opq
		if header.Typeflag == tar.TypeDir && header.PAXRecords[schilyOpaqueXattr] == "y" {
			// Write directory entry, without the xattr.
			delete(header.PAXRecords, schilyOpaqueXattr)
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			// Write opaque marker file inside the directory.
			trimmedName := strings.TrimSuffix(header.Name, string(filepath.Separator))
			opqName := trimmedName + string(filepath.Separator) + aufsOpaqueMarker
			if err := tw.WriteHeader(&tar.Header{
				Typeflag:   tar.TypeReg,
				Name:       opqName,
				Size:       0,
				Mode:       0o600,
				Uid:        header.Uid,
				Gid:        header.Gid,
				Uname:      header.Uname,
				Gname:      header.Gname,
				AccessTime: header.AccessTime,
				ChangeTime: header.ChangeTime,
			}); err != nil {
				return err
			}
			continue
		}

		// <file> as 0:0 char dev -> becomes .wh..wh.<file>
		if header.Typeflag == tar.TypeChar && header.Devmajor == 0 && header.Devminor == 0 {
			parent, base := filepath.Split(header.Name)
			header.Typeflag = tar.TypeReg
			header.Name = parent + aufsWhiteoutPrefix + base
			header.Size = 0
			header.Mode = 0o600
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		// Disable gosec G110: Potential DoS vulnerability via decompression bomb.
		// We are just filtering a flow directly from tar reader to tar writer - we aren't reading
		// into memory beyond the stdlib buffering.
		//nolint:gosec
		if _, err := io.Copy(tw, tr); err != nil {
			return err
		}
	}
}
