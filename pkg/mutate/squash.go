// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type shadow struct {
	exact    bool // If true, entry affects the exact name.
	children bool // If true, entry affects children of name.
}

type entry struct {
	hdr      *tar.Header
	shadowed bool   // If true, the named path modified/removed by a later changeset.
	b        []byte // If shadowed is true, stores content.
}

type imageState struct {
	tw *tar.Writer

	// The collective effect of entries and whiteouts from upper layers.
	imageShadows map[string]shadow

	// Hard links for which the target name has not been found. As each layer is committed,
	// relevant entries are processed and added to the underlying TAR stream.
	imageLinks map[string][]entry

	// Whiteouts from the current layer. As each layer is committed, these are merged into
	// imageShadows.
	layerWhiteouts map[string]shadow

	// Entries from the current layer that are not directories, hard links or whiteouts.
	layerEntries []entry
}

// writeChangesetEntry writes a changeset entry, which add/modify/remove image content.
func (s *imageState) writeChangesetEntry(hdr *tar.Header, r io.Reader) error {
	// If entry is a whiteout, record it.
	if base := filepath.Base(hdr.Name); strings.HasPrefix(base, aufsWhiteoutPrefix) {
		opaque := base == aufsOpaqueMarker

		// Determine the name.
		name := filepath.Dir(hdr.Name)
		if !opaque {
			name = filepath.Join(name, strings.TrimPrefix(base, aufsWhiteoutPrefix))
		}

		// Set/update image entry depending on the type of whiteout.
		is := s.layerWhiteouts[name]
		is.children = true
		if !opaque {
			is.exact = true
		}
		s.layerWhiteouts[name] = is

		return nil
	}

	name := filepath.Clean(hdr.Name)

	hdr.Name = name
	if hdr.Typeflag == tar.TypeDir {
		// The directory name in the name field should end with a slash.
		hdr.Name = name + string(filepath.Separator)
	}

	shadowed := s.isShadowed(name)

	// If entry is a hard link, set it aside; we can't write it until its target is committed.
	if hdr.Typeflag == tar.TypeLink {
		s.imageShadows[name] = shadow{
			exact:    true,
			children: true,
		}

		s.imageLinks[hdr.Linkname] = append(s.imageLinks[hdr.Linkname], entry{
			hdr:      hdr,
			shadowed: shadowed,
		})

		return nil
	}

	// If the entry isn't shadowed, copy to TAR stream.
	if !shadowed {
		if err := s.tw.WriteHeader(hdr); err != nil {
			return err
		}

		if n := hdr.Size; n > 0 {
			if _, err := io.CopyN(s.tw, r, n); err != nil {
				return err
			}
		}

		s.imageShadows[name] = shadow{
			exact:    true,
			children: hdr.Typeflag != tar.TypeDir,
		}
	}

	// One or more hard links may reference a non-directory entry, so make note of it for
	// processing when the layer is committed.
	if hdr.Typeflag != tar.TypeDir {
		e := entry{
			hdr:      hdr,
			shadowed: shadowed,
		}

		// If the entry was shadowed, temporarily store the contents; a hard link may still
		// reference the contents.
		if n := hdr.Size; shadowed && n > 0 {
			e.b = make([]byte, n)
			if _, err := io.ReadFull(r, e.b); err != nil {
				return err
			}
		}

		s.layerEntries = append(s.layerEntries, e)
	}

	return nil
}

// isShadowed returns true if name is modified/removed by a later changeset.
func (s *imageState) isShadowed(name string) bool {
	// Have we seen a modification or removal of this exact name?
	if is, ok := s.imageShadows[name]; ok && is.exact {
		return true
	}

	// Is this name shadowed by a parent?
	for name != "." {
		dir := filepath.Dir(name)
		if is, ok := s.imageShadows[dir]; ok && is.children {
			return true
		}
		name = dir
	}

	return false
}

// commitChangeset is called each time we are done processing a changeset.
func (s *imageState) commitChangeset() error {
	// Merge the effects of whiteouts in this layer to imageShadows.
	for name, wh := range s.layerWhiteouts {
		wh.exact = wh.exact || s.imageShadows[name].exact
		wh.children = wh.children || s.imageShadows[name].children
		s.imageShadows[name] = wh
	}
	s.layerWhiteouts = make(map[string]shadow)

	// Write any hard links that reference content in this layer.
	for _, e := range s.layerEntries {
		if _, err := s.writeHardlinksFor(e.hdr.Name, e); err != nil {
			return err
		}
	}
	s.layerEntries = nil

	return nil
}

// writeHardlinksFor evaluates all hard links that point to e through target name, either directly
// or transitively.
func (s *imageState) writeHardlinksFor(target string, root entry) (entry, error) {
	// Process each link that references target.
	links := s.imageLinks[target]
	delete(s.imageLinks, target)
	for _, link := range links {
		if !link.shadowed {
			// The link needs to be written to the TAR stream. If the content root (the link
			// target) isn't in the TAR stream, transform this link into the root.
			if root.shadowed {
				link.hdr.Typeflag = root.hdr.Typeflag
				link.hdr.Linkname = root.hdr.Linkname
				link.hdr.Size = root.hdr.Size
				link.hdr.Devmajor = root.hdr.Devmajor
				link.hdr.Devminor = root.hdr.Devminor

				// If extended header records are present, copy those over.
				if len(root.hdr.PAXRecords) > 0 {
					link.hdr.PAXRecords = root.hdr.PAXRecords
					link.hdr.Format = tar.FormatPAX
				}

				link.b = root.b

				root = link
			}

			if err := s.tw.WriteHeader(link.hdr); err != nil {
				return root, err
			}

			if n := link.hdr.Size; n > 0 {
				if _, err := io.CopyN(s.tw, bytes.NewReader(link.b), n); err != nil {
					return root, err
				}
			}
		}

		// Write links that point to root through this link.
		var err error
		root, err = s.writeHardlinksFor(link.hdr.Name, root)
		if err != nil {
			return root, err
		}
	}

	return root, nil
}

// squash writes a single, squashed TAR layer built from layers selected by s from img to w.
func squash(img v1.Image, s layerSelector, w io.Writer) error {
	ls, err := s.layersSelected(img)
	if err != nil {
		return fmt.Errorf("selecting layers: %w", err)
	}

	tw := tar.NewWriter(w)
	defer tw.Close()

	is := imageState{
		tw:             tw,
		imageShadows:   make(map[string]shadow),
		imageLinks:     make(map[string][]entry),
		layerWhiteouts: make(map[string]shadow),
	}

	for i := len(ls) - 1; i >= 0; i-- {
		rc, err := ls[i].Uncompressed()
		if err != nil {
			return fmt.Errorf("retrieving layer reader: %w", err)
		}
		defer rc.Close()

		tr := tar.NewReader(rc)
		for {
			hdr, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("reading layer entry: %w", err)
			}

			if err := is.writeChangesetEntry(hdr, tr); err != nil {
				return fmt.Errorf("writing layer entry: %w", err)
			}
		}

		if err := is.commitChangeset(); err != nil {
			return fmt.Errorf("finalizing layer: %w", err)
		}
	}

	return nil
}

// squashSelected replaces the layers selected by s in the base image with a single, squashed
// layer.
func squashSelected(base v1.Image, s layerSelector) (v1.Image, error) {
	opener := func() (io.ReadCloser, error) {
		pr, pw := io.Pipe()

		go func() {
			pw.CloseWithError(squash(base, s, pw))
		}()

		return pr, nil
	}

	l, err := tarball.LayerFromOpener(opener)
	if err != nil {
		return nil, err
	}

	return Apply(base, replaceSelectedLayers(s, l))
}

// Squash replaces all layers in the base image with a single, squashed layer.
func Squash(base v1.Image) (v1.Image, error) {
	return squashSelected(base, nil)
}

// SquashSubset replaces the layers starting at start index and up to (but not including) end index
// with a single, squashed layer.
func SquashSubset(base v1.Image, start, end int) (v1.Image, error) {
	return squashSelected(base, rangeLayerSelector(start, end))
}
