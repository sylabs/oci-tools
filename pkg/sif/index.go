// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sylabs/sif/v2/pkg/sif"
)

var _ v1.ImageIndex = (*imageIndex)(nil)

// ImageIndexFromFileImage returns a v1.ImageIndex corresponding to f.
func ImageIndexFromFileImage(fi *sif.FileImage) (v1.ImageIndex, error) {
	f := &fileImage{fi}

	return f.ImageIndex()
}

type imageIndex struct {
	f           *fileImage
	desc        *v1.Descriptor
	rawManifest []byte
}

// ImageIndex returns a v1.ImageIndex from f.
func (f *fileImage) ImageIndex() (v1.ImageIndex, error) {
	d, err := f.GetDescriptor(
		sif.WithDataType(sif.DataOCIRootIndex),
	)
	if err != nil {
		return nil, err
	}

	b, err := d.GetData()
	if err != nil {
		return nil, err
	}

	digest, size, err := v1.SHA256(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	return &imageIndex{
		f: f,
		desc: &v1.Descriptor{
			MediaType: types.OCIImageIndex,
			Size:      size,
			Digest:    digest,
		},
		rawManifest: b,
	}, nil
}

// MediaType of this image's manifest.
func (ix *imageIndex) MediaType() (types.MediaType, error) {
	return ix.desc.MediaType, nil
}

// Digest returns the sha256 of this index's manifest.
func (ix *imageIndex) Digest() (v1.Hash, error) {
	return ix.desc.Digest, nil
}

// Size returns the size of the manifest.
func (ix *imageIndex) Size() (int64, error) {
	return ix.desc.Size, nil
}

// IndexManifest returns this image index's manifest object.
func (ix *imageIndex) IndexManifest() (*v1.IndexManifest, error) {
	var im v1.IndexManifest
	err := json.Unmarshal(ix.rawManifest, &im)
	return &im, err
}

// RawManifest returns the serialized bytes of IndexManifest().
func (ix *imageIndex) RawManifest() ([]byte, error) {
	return ix.rawManifest, nil
}

// Descriptor returns the original descriptor from an index manifest. See partial.Descriptor.
func (ix *imageIndex) Descriptor() (*v1.Descriptor, error) {
	return ix.desc, nil
}

var errUnexpectedMediaType = errors.New("unexpected media type")

// Image returns a v1.Image that this ImageIndex references.
func (ix *imageIndex) Image(h v1.Hash) (v1.Image, error) {
	desc, err := ix.findDescriptor(h)
	if err != nil {
		return nil, err
	}

	if mt := desc.MediaType; !mt.IsImage() {
		return nil, fmt.Errorf("%w for %v: %v", errUnexpectedMediaType, h, desc.MediaType)
	}

	b, err := ix.f.Bytes(h)
	if err != nil {
		return nil, err
	}

	img := image{
		f:           ix.f,
		desc:        desc,
		rawManifest: b,
	}
	return &img, nil
}

// ImageIndex returns a v1.ImageIndex that this ImageIndex references.
func (ix *imageIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	desc, err := ix.findDescriptor(h)
	if err != nil {
		return nil, err
	}

	if mt := desc.MediaType; !mt.IsIndex() {
		return nil, fmt.Errorf("%w for %v: %v", errUnexpectedMediaType, h, desc.MediaType)
	}

	b, err := ix.f.Bytes(h)
	if err != nil {
		return nil, err
	}

	return &imageIndex{
		f:           ix.f,
		desc:        desc,
		rawManifest: b,
	}, nil
}

var errDescriptorNotFoundInIndex = errors.New("descriptor not found in index")

// findDescriptor returns the descriptor with the supplied digest.
func (ix *imageIndex) findDescriptor(h v1.Hash) (*v1.Descriptor, error) {
	im, err := ix.IndexManifest()
	if err != nil {
		return nil, err
	}

	for _, desc := range im.Manifests {
		if desc.Digest == h {
			return &desc, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", errDescriptorNotFoundInIndex, h)
}
