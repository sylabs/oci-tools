// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	ocimutate "github.com/sylabs/oci-tools/pkg/mutate"
)

// Use a fixed digest, so that this is repeatable.
//
//nolint:gochecknoglobals, lll
var helloWorldRef = name.MustParseReference("hello-world@sha256:00e1ee7c898a2c393ea2fe7680938f8dcbe55e51fbf08032cf37326a677f92ed")

func generateImages(path string) error {
	platform := v1.Platform{
		Architecture: "arm64",
		OS:           "linux",
		Variant:      "v8",
	}

	images := []struct {
		source      name.Reference
		opts        []remote.Option
		destination string
	}{
		{
			source:      helloWorldRef,
			destination: filepath.Join(path, "hello-world-docker-v2-manifest"),
			opts: []remote.Option{
				remote.WithPlatform(platform),
			},
		},
	}

	for _, im := range images {
		img, err := remote.Image(im.source, im.opts...)
		if err != nil {
			return err
		}

		lp, err := layout.Write(im.destination, empty.Index)
		if err != nil {
			return err
		}

		if err := lp.AppendImage(img); err != nil {
			return err
		}
	}

	return nil
}

func generateIndexes(path string) error {
	images := []struct {
		source      name.Reference
		opts        []remote.Option
		destination string
	}{
		{
			source:      helloWorldRef,
			destination: filepath.Join(path, "hello-world-docker-v2-manifest-list"),
		},
	}

	for _, im := range images {
		ii, err := remote.Index(im.source, im.opts...)
		if err != nil {
			return err
		}

		lp, err := layout.Write(im.destination, empty.Index)
		if err != nil {
			return err
		}

		if err := lp.AppendIndex(ii); err != nil {
			return err
		}
	}

	return nil
}

type tarEntry struct {
	Typeflag   byte
	Name       string
	Linkname   string
	Content    string
	PAXRecords map[string]string
}

func (e tarEntry) Header() *tar.Header {
	h := &tar.Header{
		Typeflag: e.Typeflag,
		Name:     e.Name,
		Linkname: e.Linkname,
		Mode:     0o555,
		Size:     int64(len(e.Content)),
		Format:   tar.FormatGNU,
	}
	if e.Typeflag == tar.TypeDir {
		h.Mode = 0o755
	}
	if len(e.PAXRecords) > 0 {
		h.PAXRecords = e.PAXRecords
		h.Format = tar.FormatPAX
	}

	return h
}

func writeLayerTAR(w io.Writer, tes []tarEntry) error {
	tw := tar.NewWriter(w)

	for _, te := range tes {
		if err := tw.WriteHeader(te.Header()); err != nil {
			return err
		}

		if te.Content != "" {
			if _, err := io.WriteString(tw, te.Content); err != nil {
				return err
			}
		}
	}

	return tw.Close()
}

//nolint:maintidx
func generateTARImages(path string) error {
	images := []struct {
		layers      [][]tarEntry
		destination string
	}{
		// Image with explicit root directory "./".
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "./"},
				},
			},
			destination: filepath.Join(path, "root-dir-entry"),
		},
		// Image with explicit whiteout of file "a/b/foo". Implied contents:
		//
		//	a/
		//	a/b
		//	a/b/bar
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/.wh.foo"},
					{Typeflag: tar.TypeReg, Name: "a/b/bar", Content: "bar"},
				},
			},
			destination: filepath.Join(path, "whiteout-explicit-file"),
		},
		// Image with explicit whiteout of directory "a/b/". Implied contents:
		//
		//	a/
		//	a/bar
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeReg, Name: "a/.wh.b"},
					{Typeflag: tar.TypeReg, Name: "a/bar", Content: "bar"},
				},
			},
			destination: filepath.Join(path, "whiteout-explicit-dir"),
		},
		// Image with opaque whiteout of directory "a/". Implied contents:
		//
		//	a/
		//	a/bar
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeReg, Name: "a/.wh..wh..opq"},
					{Typeflag: tar.TypeReg, Name: "a/bar", Content: "bar"},
				},
			},
			destination: filepath.Join(path, "whiteout-opaque"),
		},
		// Image with opaque whiteout of directory "a/" at the end of the layer. Implied contents:
		//
		//	a/
		//	a/bar
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeReg, Name: "a/bar", Content: "bar"},
					{Typeflag: tar.TypeReg, Name: "a/.wh..wh..opq"},
				},
			},
			destination: filepath.Join(path, "whiteout-opaque-end"),
		},
		// Image with a hard link to a regular file. Implied contents:
		//
		//	a/
		//	a/b/
		//	a/b/foo
		//  a/b/bar => a/b/foo
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo"},
					{Typeflag: tar.TypeLink, Name: "a/b/bar", Linkname: "a/b/foo"},
				},
			},
			destination: filepath.Join(path, "hard-link-1"),
		},
		// Image with a hard link to a regular file in a different layer. Implied contents:
		//
		//	a/
		//	a/b/
		//	a/b/foo
		//  a/b/bar => a/b/foo
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeLink, Name: "a/b/bar", Linkname: "a/b/foo"},
				},
			},
			destination: filepath.Join(path, "hard-link-2"),
		},
		// Image with a hard link to a deleted regular file. Implied contents:
		//
		//	a/
		//	a/b/
		//  a/b/bar
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo"},
					{Typeflag: tar.TypeLink, Name: "a/b/bar", Linkname: "a/b/foo"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/.wh.foo"},
				},
			},
			destination: filepath.Join(path, "hard-link-delete-1"),
		},
		// Image with a deleted hard link to a regular file. Implied contents:
		//
		//	a/
		//	a/b/
		//	a/b/foo
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo"},
					{Typeflag: tar.TypeLink, Name: "a/b/bar", Linkname: "a/b/foo"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/.wh.bar"},
				},
			},
			destination: filepath.Join(path, "hard-link-delete-2"),
		},
		// Image with a hard link chain to a deleted regular file. Implied contents:
		//
		//	a/
		//	a/b/
		//	a/b/bar
		//	a/b/baz => a/b/bar
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo"},
					{Typeflag: tar.TypeLink, Name: "a/b/bar", Linkname: "a/b/foo"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeLink, Name: "a/b/baz", Linkname: "a/b/bar"},
					{Typeflag: tar.TypeReg, Name: "a/b/.wh.foo"},
				},
			},
			destination: filepath.Join(path, "hard-link-delete-3"),
		},
		// Image with a hard link chain to a deleted regular file. Implied contents:
		//
		//	a/
		//	a/b/
		//	a/b/foo
		//	a/b/baz => a/b/foo
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo"},
					{Typeflag: tar.TypeLink, Name: "a/b/bar", Linkname: "a/b/foo"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeLink, Name: "a/b/baz", Linkname: "a/b/bar"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/.wh.bar"},
				},
			},
			destination: filepath.Join(path, "hard-link-delete-4"),
		},
		// Image with a hard link to a deleted regular file with extended attributes. Implied
		// contents:
		//
		//	a/
		//	a/b/
		//  a/b/bar (with extended attributes)
		{
			layers: [][]tarEntry{
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/foo", Content: "foo", PAXRecords: map[string]string{
						"SCHILY.xattr.user.foo": "bar",
					}},
					{Typeflag: tar.TypeLink, Name: "a/b/bar", Linkname: "a/b/foo"},
				},
				{
					{Typeflag: tar.TypeDir, Name: "a/"},
					{Typeflag: tar.TypeDir, Name: "a/b/"},
					{Typeflag: tar.TypeReg, Name: "a/b/.wh.foo"},
				},
			},
			destination: filepath.Join(path, "hard-link-delete-xattr"),
		},
	}

	for _, im := range images {
		img := empty.Image

		for _, layer := range im.layers {
			opener := func() (io.ReadCloser, error) {
				pr, pw := io.Pipe()
				go func() {
					pw.CloseWithError(writeLayerTAR(pw, layer))
				}()
				return pr, nil
			}

			l, err := tarball.LayerFromOpener(opener)
			if err != nil {
				return err
			}

			img, err = mutate.AppendLayers(img, l)
			if err != nil {
				return err
			}
		}

		lp, err := layout.Write(im.destination, empty.Index)
		if err != nil {
			return err
		}

		if err := lp.AppendImage(img); err != nil {
			return err
		}
	}

	return nil
}

func generateManyLayerImage(path string) error {
	// Generate 50 unique layers.
	ls := make([]v1.Layer, 50)

	for i := range ls {
		layer := []tarEntry{
			{Typeflag: tar.TypeReg, Name: strconv.Itoa(i)},
		}

		opener := func() (io.ReadCloser, error) {
			pr, pw := io.Pipe()
			go func() {
				pw.CloseWithError(writeLayerTAR(pw, layer))
			}()
			return pr, nil
		}

		l, err := tarball.LayerFromOpener(opener)
		if err != nil {
			return err
		}

		ls[i] = l
	}

	img, err := mutate.AppendLayers(empty.Image, ls...)
	if err != nil {
		return err
	}

	lp, err := layout.Write(filepath.Join(path, "many-layers"), empty.Index)
	if err != nil {
		return err
	}

	return lp.AppendImage(img)
}

var errMultipleImages = errors.New("multiple images found in index")

func generateSquashFSImages(path string) error {
	images := []struct {
		source       string
		destination  string
		squashLayers bool
	}{
		{
			source:       filepath.Join(path, "aufs-docker-v2-manifest"),
			destination:  filepath.Join(path, "overlayfs-docker-v2-manifest"),
			squashLayers: false, // need to preserve whiteout test files
		},
	}

	for _, im := range images {
		ii, err := layout.ImageIndexFromPath(im.source)
		if err != nil {
			return err
		}

		ix, err := ii.IndexManifest()
		if err != nil {
			return err
		}
		if len(ix.Manifests) != 1 {
			return errMultipleImages
		}

		ih := ix.Manifests[0].Digest
		img, err := ii.Image(ih)
		if err != nil {
			return err
		}

		if im.squashLayers {
			img, err = ocimutate.Squash(img)
			if err != nil {
				return err
			}
		}

		ms := []ocimutate.Mutation{}
		ls, err := img.Layers()
		if err != nil {
			return err
		}

		for i, l := range ls {
			squashfsLayer, err := ocimutate.SquashfsLayer(l, os.TempDir())
			if err != nil {
				return err
			}
			ms = append(ms, ocimutate.SetLayer(i, squashfsLayer))
		}

		img, err = ocimutate.Apply(img, ms...)
		if err != nil {
			return err
		}

		lp, err := layout.Write(im.destination, empty.Index)
		if err != nil {
			return err
		}

		if err := lp.AppendImage(img); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	path := "."
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	if err := generateImages(path); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if err := generateIndexes(path); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if err := generateTARImages(path); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if err := generateManyLayerImage(path); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if err := generateSquashFSImages(path); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
