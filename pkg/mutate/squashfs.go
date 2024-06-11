// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

const squashfsLayerMediaType types.MediaType = "application/vnd.sylabs.image.layer.v1.squashfs"

type squashfsConverter struct {
	converter       string   // Path to converter program.
	args            []string // Arguments required for converter program.
	dir             string   // Working directory.
	convertWhiteout bool     // Convert whiteout markers from AUFS -> OverlayFS
}

// SquashfsConverterOpt are used to specify squashfs converter options.
type SquashfsConverterOpt func(*squashfsConverter) error

// OptSquashfsLayerConverter specifies the converter program to use when converting from TAR to
// SquashFS format.
func OptSquashfsLayerConverter(converter string) SquashfsConverterOpt {
	return func(c *squashfsConverter) error {
		path, err := exec.LookPath(converter)
		if err != nil {
			return err
		}

		c.converter = path

		return nil
	}
}

var errSquashfsConverterNotSupported = errors.New("squashfs converter not supported")

// OptSquashfsSkipWhiteoutConversion is set to skip the default conversion of whiteout /
// opaque markers from AUFS to OverlayFS format.
func OptSquashfsSkipWhiteoutConversion(b bool) SquashfsConverterOpt {
	return func(c *squashfsConverter) error {
		c.convertWhiteout = !b
		return nil
	}
}

// SquashfsLayer converts the base layer into a layer using the squashfs format. A dir must be
// specified, which is used as a working directory during conversion. The caller is responsible for
// cleaning up dir.
//
// By default, this will attempt to locate a suitable TAR to SquashFS converter such as 'tar2sqfs'
// or `sqfstar` via exec.LookPath. To specify a path to a specific converter program, consider
// using OptSquashfsLayerConverter.
//
// By default, AUFS whiteout markers in the base TAR layer will be converted to OverlayFS whiteout
// markers in the SquashFS layer. This can be disabled, e.g. where it is known that the layer is
// part of a squashed image that will not have any whiteouts, using OptSquashfsSkipWhiteoutConversion.
//
// Note - when whiteout conversion is performed the base layer will be read twice. Callers should
// ensure it is cached, and is not a streaming layer.
func SquashfsLayer(base v1.Layer, dir string, opts ...SquashfsConverterOpt) (v1.Layer, error) {
	c := squashfsConverter{
		dir:             dir,
		convertWhiteout: true,
	}

	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return nil, err
		}
	}

	if c.converter == "" {
		path, err := exec.LookPath("tar2sqfs")
		if err != nil {
			if path, err = exec.LookPath("sqfstar"); err != nil {
				return nil, err
			}
		}

		c.converter = path
	}

	switch base := filepath.Base(c.converter); base {
	case "tar2sqfs":
		// Use gzip compression instead of the default (xz).
		c.args = []string{
			"--compressor", "gzip",
		}

	case "sqfstar":
		// The `sqfstar` binary by default creates a root directory that is owned by the
		// uid/gid of the user running it, and uses the current time for the root directory
		// inode as well as the modification_time field of the superblock.
		//
		// The options below modify this behaviour to instead use predictable values, but
		// unfortunately they do not function correctly with squashfs-tools v4.5.
		c.args = []string{
			"-mkfs-time", "0",
			"-root-time", "0",
			"-root-uid", "0",
			"-root-gid", "0",
			"-root-mode", "0755",
		}

	default:
		return nil, fmt.Errorf("%v: %w", base, errSquashfsConverterNotSupported)
	}

	return c.layer(base)
}

// makeSquashfs returns the path to a squashfs file that contains the contents of the uncompressed
// TAR stream from r.
func (c *squashfsConverter) makeSquashfs(r io.Reader) (string, error) {
	dir, err := os.MkdirTemp(c.dir, "")
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, "layer.sqfs")

	//nolint:gosec // Arguments are created programatically.
	cmd := exec.Command(c.converter, append(c.args, path)...)
	cmd.Stdin = r

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s error: %w, output: %s", c.converter, err, out)
	}

	return path, nil
}

// Uncompressed returns an io.ReadCloser for the uncompressed layer contents. If
// c.convertWhiteout is true it will convert whiteout markers from AUFS ->
// OverlayFS format. Note that when conversion is performed, the underlying
// layer TAR is read twice.
func (c *squashfsConverter) Uncompressed(l v1.Layer) (io.ReadCloser, error) {
	rc, err := l.Uncompressed()
	if err != nil {
		return nil, err
	}

	// No conversion - direct tar stream from the layer.
	if !c.convertWhiteout {
		return rc, nil
	}

	// Conversion - first, scan for opaque directories and presence of file
	// whiteout markers.
	opaquePaths, fileWhiteout, err := scanAUFSWhiteouts(rc)
	if err != nil {
		return nil, err
	}
	rc.Close()

	rc, err = l.Uncompressed()
	if err != nil {
		return nil, err
	}

	// Nothing found to filter
	if len(opaquePaths) == 0 && !fileWhiteout {
		return rc, nil
	}

	pr, pw := io.Pipe()
	go func() {
		defer rc.Close()
		pw.CloseWithError(whiteoutsToOverlayFS(rc, pw, opaquePaths))
	}()
	return pr, nil
}

type squashfsLayer struct {
	base      v1.Layer
	converter *squashfsConverter

	computed bool
	path     string
	hash     v1.Hash
	size     int64

	sync.Mutex
}

var errUnsupportedLayerType = errors.New("unsupported layer type")

// layer converts base to squashfs format.
func (c *squashfsConverter) layer(base v1.Layer) (v1.Layer, error) {
	mt, err := base.MediaType()
	if err != nil {
		return nil, err
	}

	//nolint:exhaustive // Exhaustive cases not appropriate.
	switch mt {
	case squashfsLayerMediaType:
		return base, nil

	case types.DockerLayer, types.DockerUncompressedLayer, types.OCILayer, types.OCIUncompressedLayer:
		return &squashfsLayer{
			base:      base,
			converter: c,
		}, nil

	default:
		return nil, fmt.Errorf("%w: %v", errUnsupportedLayerType, mt)
	}
}

// populate populates various fields in l.
func (l *squashfsLayer) populate() error {
	l.Lock()
	defer l.Unlock()

	if l.computed {
		return nil
	}

	rc, err := l.converter.Uncompressed(l.base)
	if err != nil {
		return err
	}
	defer rc.Close()

	path, err := l.converter.makeSquashfs(rc)
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h, n, err := v1.SHA256(f)
	if err != nil {
		return err
	}

	l.computed = true
	l.path = path
	l.hash = h
	l.size = n

	return nil
}

// Digest returns the Hash of the compressed layer.
func (l *squashfsLayer) Digest() (v1.Hash, error) {
	return l.DiffID()
}

// DiffID returns the Hash of the uncompressed layer.
func (l *squashfsLayer) DiffID() (v1.Hash, error) {
	if err := l.populate(); err != nil {
		return v1.Hash{}, err
	}

	return l.hash, nil
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
func (l *squashfsLayer) Compressed() (io.ReadCloser, error) {
	return l.Uncompressed()
}

// Uncompressed returns an io.ReadCloser for the uncompressed layer contents.
func (l *squashfsLayer) Uncompressed() (io.ReadCloser, error) {
	if err := l.populate(); err != nil {
		return nil, err
	}

	return os.Open(l.path)
}

// Size returns the compressed size of the Layer.
func (l *squashfsLayer) Size() (int64, error) {
	if err := l.populate(); err != nil {
		return 0, err
	}

	return l.size, nil
}

// MediaType returns the media type of the Layer.
func (l *squashfsLayer) MediaType() (types.MediaType, error) {
	return squashfsLayerMediaType, nil
}
