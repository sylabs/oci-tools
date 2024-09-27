// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"bytes"
	"errors"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sylabs/sif/v2/pkg/sif"
)

// WriteBlob writes a blob to the SIF f, as a DataOCIBlob descriptor.
func (f *OCIFileImage) WriteBlob(r io.Reader) error {
	return f.writeBlob(r, sif.DataOCIBlob)
}

// writeRootIndex writes the RootIndex blob to the SIF f, as a
// DataOCIRootIndex descriptor.
func (f *OCIFileImage) writeRootIndex(r io.Reader) error {
	return f.writeBlob(r, sif.DataOCIRootIndex)
}

func (f *OCIFileImage) writeBlob(r io.Reader, t sif.DataType) error {
	di, err := sif.NewDescriptorInput(t, r)
	if err != nil {
		return err
	}

	return f.sif.AddObject(di)
}

// writeImage writes an image and all of its manifests and blobs to f. This
// function does not update the RootIndex.
func (f *OCIFileImage) writeImage(img v1.Image) error {
	ls, err := img.Layers()
	if err != nil {
		return err
	}

	for _, l := range ls {
		rc, err := l.Compressed()
		if err != nil {
			return err
		}

		if err := f.WriteBlob(rc); err != nil {
			return err
		}
	}

	cfg, err := img.RawConfigFile()
	if err != nil {
		return err
	}

	if err := f.WriteBlob(bytes.NewReader(cfg)); err != nil {
		return err
	}

	rm, err := img.RawManifest()
	if err != nil {
		return err
	}

	return f.WriteBlob(bytes.NewReader(rm))
}

type withBlob interface {
	Blob(v1.Hash) (io.ReadCloser, error)
}

type withLayer interface {
	Layer(v1.Hash) (v1.Layer, error)
}

var errUnableToReadBlob = errors.New("unable to read blob from index")

// blobFromIndex returns a ReadCloser corresponding to the digest. Unfortunately, the v1.ImageIndex
// does not expose arbitrary blobs (https://github.com/google/go-containerregistry/issues/819) so
// attempt to work around that.
func blobFromIndex(ii v1.ImageIndex, digest v1.Hash) (io.ReadCloser, error) {
	if wb, ok := ii.(withBlob); ok {
		return wb.Blob(digest)
	}

	if wl, ok := ii.(withLayer); ok {
		l, err := wl.Layer(digest)
		if err != nil {
			return nil, err
		}

		return l.Compressed()
	}

	return nil, errUnableToReadBlob
}

// writeIndex writes an index and all of its child indexes, manifests and blobs
// to f.
func (f *OCIFileImage) writeIndex(ii v1.ImageIndex, rootIndex bool) error {
	index, err := ii.IndexManifest()
	if err != nil {
		return err
	}

	for _, desc := range index.Manifests {
		//nolint:exhaustive // Exhaustive cases not appropriate.
		switch desc.MediaType {
		case types.DockerManifestList, types.OCIImageIndex:
			ii, err := ii.ImageIndex(desc.Digest)
			if err != nil {
				return err
			}

			if err := f.writeIndex(ii, false); err != nil {
				return err
			}

		case types.DockerManifestSchema2, types.OCIManifestSchema1:
			img, err := ii.Image(desc.Digest)
			if err != nil {
				return err
			}

			if err := f.writeImage(img); err != nil {
				return err
			}

		default:
			rc, err := blobFromIndex(ii, desc.Digest)
			if err != nil {
				return err
			}
			defer rc.Close()

			if err := f.WriteBlob(rc); err != nil {
				return err
			}
		}
	}

	m, err := ii.RawManifest()
	if err != nil {
		return err
	}

	if rootIndex {
		return f.writeRootIndex(bytes.NewReader(m))
	}

	return f.WriteBlob(bytes.NewReader(m))
}

// numDescriptorsForImage returns the number of descriptors required to store img.
func numDescriptorsForImage(img v1.Image) (int64, error) {
	ls, err := img.Layers()
	if err != nil {
		return 0, err
	}

	return int64(len(ls) + 2), nil
}

// numDescriptorsForIndex returns the number of descriptors required to store ii.
func numDescriptorsForIndex(ii v1.ImageIndex) (int64, error) {
	index, err := ii.IndexManifest()
	if err != nil {
		return 0, err
	}

	var count int64

	for _, desc := range index.Manifests {
		//nolint:exhaustive // Exhaustive cases not appropriate.
		switch desc.MediaType {
		case types.DockerManifestList, types.OCIImageIndex:
			ii, err := ii.ImageIndex(desc.Digest)
			if err != nil {
				return 0, err
			}

			n, err := numDescriptorsForIndex(ii)
			if err != nil {
				return 0, err
			}

			count += n

		case types.DockerManifestSchema2, types.OCIManifestSchema1:
			img, err := ii.Image(desc.Digest)
			if err != nil {
				return 0, err
			}

			n, err := numDescriptorsForImage(img)
			if err != nil {
				return 0, err
			}

			count += n

		default:
			count++
		}
	}

	return count + 1, nil
}

// writeOpts accumulates write options.
type writeOpts struct {
	spareDescriptors int64
}

// WriteOpt are used to specify write options.
type WriteOpt func(*writeOpts) error

// OptWriteWithSpareDescriptorCapacity specifies that the SIF should be created with n spare
// descriptors.
func OptWriteWithSpareDescriptorCapacity(n int64) WriteOpt {
	return func(wo *writeOpts) error {
		wo.spareDescriptors = n
		return nil
	}
}

// Write constructs a SIF at path from an ImageIndex, which becomes the
// RootIndex in the SIF.
//
// By default, the SIF is created with the exact number of descriptors required
// to represent ii. To include spare descriptor capacity, consider using
// OptWriteWithSpareDescriptorCapacity.
func Write(path string, ii v1.ImageIndex, opts ...WriteOpt) error {
	wo := writeOpts{
		spareDescriptors: 0,
	}

	for _, opt := range opts {
		if err := opt(&wo); err != nil {
			return err
		}
	}

	n, err := numDescriptorsForIndex(ii)
	if err != nil {
		return err
	}

	fi, err := sif.CreateContainerAtPath(path,
		sif.OptCreateDeterministic(),
		sif.OptCreateWithDescriptorCapacity(n+wo.spareDescriptors),
	)
	if err != nil {
		return err
	}
	defer func() { _ = fi.UnloadContainer() }()

	f := OCIFileImage{fi}

	return f.writeIndex(ii, true)
}
