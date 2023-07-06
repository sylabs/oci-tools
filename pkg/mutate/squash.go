// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"archive/tar"
	"io"
	"strings"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
	ssimg "github.com/anchore/stereoscope/pkg/image"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Squash replaces the layers in the base image with a single, squashed layer.
func Squash(base v1.Image) (v1.Image, error) {
	l, err := tarball.LayerFromOpener(tarFromStereoscope(base))
	if err != nil {
		return nil, err
	}

	return Apply(base, ReplaceLayers(l))
}

// walkFn is the squashedtree walk function used to parse the file tree.
type walkFn func(file.Path, filenode.FileNode) error

// getTARWalker returns a func that writes each FileNode from img into tw.
func getTARWalker(img *ssimg.Image, tw *tar.Writer) walkFn {
	return func(path file.Path, n filenode.FileNode) error {
		fileInfo, err := img.FileCatalog.Get(*n.Reference)
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(fileInfo, string(n.LinkPath))
		if err != nil {
			return err
		}

		hdr.Name = strings.TrimLeft(string(path), "/")
		if hdr.Typeflag == tar.TypeDir {
			hdr.Name += "/"
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if n.FileType == file.TypeRegular {
			rc, err := img.OpenPathFromSquash(path)
			if err != nil {
				return err
			}
			defer rc.Close()

			if _, err = io.CopyN(tw, rc, hdr.Size); err != nil {
				return err
			}
		}

		return nil
	}
}

func shouldVisit(p file.Path, fn filenode.FileNode) bool {
	// If the path isn't equal to its n.RealPath then its likely a
	// symlinked parent directory so no need to evaluate the file
	// short-circuit out and move on.
	if string(p) != string(fn.RealPath) {
		return false
	}

	if fn.Reference == nil {
		return false
	}

	return true
}

func shouldContinue() func(file.Path, filenode.FileNode) bool {
	avoidLoopTraversal := make(map[file.Path]int)
	return func(p file.Path, fn filenode.FileNode) bool {
		// This check prevents trying to walk a symlink linked to
		// the current directory more than once. After that it will
		// bail out and prevent re-walking the symlink again, causing
		// a traveral loop as mentioned in the issues below.
		//
		// https://github.com/anchore/stereoscope/issues/160
		// https://github.com/sylabs/panoplia/issues/279

		// If symlink is linked to current dir
		if fn.IsLink() && string(fn.LinkPath) == "." {
			avoidLoopTraversal[fn.RealPath]++
			// if we try to traverse the symlink again, bail out
			if avoidLoopTraversal[fn.RealPath] > 1 {
				return false
			}
		}

		return true
	}
}

// walkSquashedTree walks through img, calling fn with each file node.
func walkSquashedTree(img *ssimg.Image, fn walkFn) error {
	return img.SquashedTree().Walk(fn, &filetree.WalkConditions{
		LinkOptions: []filetree.LinkResolutionOption{
			filetree.DoNotFollowDeadBasenameLinks,
		},
		ShouldVisit:          shouldVisit,
		ShouldContinueBranch: shouldContinue(),
	})
}

// writeTAR writes a TAR file to w, corresponding to the squashed layers of the specified base
// image.
func writeTAR(base v1.Image, w io.Writer) error {
	gen := file.NewTempDirGenerator("stereoscope")

	dir, err := gen.NewDirectory()
	if err != nil {
		return err
	}

	si := ssimg.New(base, gen, dir)
	defer func() { _ = si.Cleanup() }()

	if err := si.Read(); err != nil {
		return err
	}

	tw := tar.NewWriter(w)
	defer tw.Close()

	return walkSquashedTree(si, getTARWalker(si, tw))
}

// tarFromStereoscope returns an Opener with the squashed contents of the base image.
func tarFromStereoscope(base v1.Image) tarball.Opener {
	return func() (io.ReadCloser, error) {
		pr, pw := io.Pipe()

		go func() {
			// Close the writer with any errors encountered during extraction. These errors will be
			// returned by the reader end on subsequent reads. If err == nil, the reader will
			// return EOF.
			pw.CloseWithError(writeTAR(base, pw))
		}()

		return pr, nil
	}
}
