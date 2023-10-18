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
)

var errUnexpectedOpaque = errors.New("unexpected opaque marker")

// scanAUFSWhiteouts reads a TAR stream, returning a map of <path>:true for
// directories in the tar that contain an AUFS .wh..wh..opq opaque directory
// marker file, and a boolean indicating the presence of any .wh.<file> markers.
// Note that paths returned are clean, per filepath.Clean.
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

		base := filepath.Base(header.Name)

		if base == aufsOpaqueMarker {
			parent := filepath.Dir(header.Name)
			opaquePaths[parent] = true
		}

		if !fileWhiteout && strings.HasPrefix(base, aufsWhiteoutPrefix) {
			fileWhiteout = true
		}
	}
}

// whiteOutFilter streams a tar file from in to out, replacing AUFS whiteout
// markers with OverlayFS whiteout markers. Due to unrestricted ordering of
// markers vs their target, the list of opaquePaths must be obtained prior to
// filtering and provided to this filter.
func whiteoutFilter(in io.ReadCloser, out io.WriteCloser, opaquePaths map[string]bool) error {
	tr := tar.NewReader(in)
	tw := tar.NewWriter(out)
	defer out.Close()
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

		clean := filepath.Clean(header.Name)
		base := filepath.Base(header.Name)
		parent := filepath.Dir(header.Name)

		// Don't include .wh..wh..opq opaque directory markers in output.
		if base == aufsOpaqueMarker {
			// If we don't know the target should be opaque, then provided opaquePaths is incorrect.
			if !opaquePaths[parent] {
				return fmt.Errorf("%q: %w", parent, errUnexpectedOpaque)
			}
			continue
		}
		// Set overlayfs xattr on a dir that was previously found to contain a .wh..wh..opq marker.
		if opq := opaquePaths[clean]; opq {
			if header.PAXRecords == nil {
				header.PAXRecords = map[string]string{}
			}
			header.PAXRecords["SCHILY.xattr."+"trusted.overlay.opaque"] = "y"
		}
		// Replace a `.wh.<name>` marker with a char dev 0 at <name>
		if strings.HasPrefix(base, aufsWhiteoutPrefix) {
			target := filepath.Join(parent, strings.TrimPrefix(base, aufsWhiteoutPrefix))
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
