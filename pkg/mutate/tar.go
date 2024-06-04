// Copyright 2024 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type tarConverter struct {
	converter       string // Path to converter program.
	dir             string // Working directory.
	convertWhiteout bool   // Convert whiteout markers from OverlayFS -> AUFS
}

// TarConverterOpt are used to specify tar converter options.
type TarConverterOpt func(*tarConverter) error

// OptTarLayerConverter specifies the converter program to use when converting from SquashFS to
// tar format.
func OptTarLayerConverter(converter string) TarConverterOpt {
	return func(c *tarConverter) error {
		path, err := exec.LookPath(converter)
		if err != nil {
			return err
		}

		c.converter = path

		return nil
	}
}

// OptTarSkipWhiteoutConversion is set to skip the default conversion of whiteout /
// opaque markers from OverlayFS to AUFS format.
func OptTarSkipWhiteoutConversion(b bool) TarConverterOpt {
	return func(c *tarConverter) error {
		c.convertWhiteout = !b
		return nil
	}
}

// TarFromSquashfsLayer returns an opener that will provide a TAR conversion of
// the SquashFS format base layer. A dir must be specified, which is used as a
// working directory during conversion. The caller is responsible for cleaning
// up dir.
//
// By default, this will attempt to locate a suitable SquashFS to tar converter,
// currently only 'sqfs2tar', via exec.LookPath. To specify a path to a specific
// converter program, consider using OptTarLayerConverter.
//
// By default, OverlayFS whiteout markers in the base SquashFS layer will be
// converted to AUFS whiteout markers in the TAR layer. This can be disabled,
// e.g. where it is known that the layer is part of a squashed image that will
// not have any whiteouts, using OptTarSkipWhiteourConversion.
func TarFromSquashfsLayer(base v1.Layer, dir string, opts ...TarConverterOpt) (tarball.Opener, error) {
	mt, err := base.MediaType()
	if err != nil {
		return nil, err
	}
	if mt != squashfsLayerMediaType {
		return nil, fmt.Errorf("%w: %v", errUnsupportedLayerType, mt)
	}

	c := tarConverter{
		dir:             dir,
		convertWhiteout: true,
	}

	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return nil, err
		}
	}

	if c.converter == "" {
		path, err := exec.LookPath("sqfs2tar")
		if err != nil {
			return nil, err
		}
		c.converter = path
	}

	return c.opener(base), nil
}

// makeSquashfs returns a the path to a TAR file that contains the contents of
// the SquashFS stream from r.
func (c *tarConverter) makeTAR(r io.Reader) (string, error) {
	dir, err := os.MkdirTemp(c.dir, "")
	if err != nil {
		return "", err
	}

	sqfsPath := filepath.Join(dir, "layer.sqfs")
	sqfsFile, err := os.Create(sqfsPath)
	if err != nil {
		return "", err
	}
	defer sqfsFile.Close()

	_, err = io.Copy(sqfsFile, r)
	if err != nil {
		return "", err
	}

	tarPath := filepath.Join(dir, "layer.tar")
	tarFile, err := os.Create(tarPath)
	if err != nil {
		return "", err
	}
	defer tarFile.Close()

	//nolint:gosec // Arguments are created programatically.
	cmd := exec.Command(c.converter, sqfsPath)
	cmd.Stdout = tarFile
	errBuff := bytes.Buffer{}
	cmd.Stderr = &errBuff

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s error: %w %v", c.converter, err, errBuff)
	}

	return tarPath, nil
}

// Opener returns a tarball.Opener that will open a TAR file holding the content
// of a SquashFS layer l, converted to TAR format.
func (c *tarConverter) opener(l v1.Layer) tarball.Opener {
	return func() (io.ReadCloser, error) {
		rc, err := l.Uncompressed()
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		tarFile, err := c.makeTAR(rc)
		if err != nil {
			return nil, err
		}

		tr, err := os.Open(tarFile)
		if err != nil {
			return nil, err
		}

		if !c.convertWhiteout {
			return tr, nil
		}

		pr, pw := io.Pipe()
		go func() {
			defer rc.Close()
			pw.CloseWithError(whiteoutsToAUFS(tr, pw))
		}()
		return pr, nil
	}
}
