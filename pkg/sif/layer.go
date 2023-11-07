// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

var _ v1.Layer = (*Layer)(nil)

type Layer struct {
	f    *fileImage
	desc v1.Descriptor
}

// Digest returns the Hash of the compressed layer.
func (l *Layer) Digest() (v1.Hash, error) {
	return l.desc.Digest, nil
}

// DiffID returns the Hash of the uncompressed layer.
func (l *Layer) DiffID() (v1.Hash, error) {
	r, err := l.Uncompressed()
	if err != nil {
		return v1.Hash{}, err
	}
	defer r.Close()

	h, _, err := v1.SHA256(r)
	return h, err
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
func (l *Layer) Compressed() (io.ReadCloser, error) {
	return l.f.Blob(l.desc.Digest)
}

// Uncompressed returns an io.ReadCloser for the uncompressed layer contents.
func (l *Layer) Uncompressed() (io.ReadCloser, error) {
	cl, err := partial.CompressedToLayer(l)
	if err != nil {
		return nil, err
	}

	return cl.Uncompressed()
}

// Size returns the compressed size of the Layer.
func (l *Layer) Size() (int64, error) {
	return l.desc.Size, nil
}

// Offset returns the offset within the SIF image of the Layer.
func (l *Layer) Offset() (int64, error) {
	return l.f.Offset(l.desc.Digest)
}

// MediaType returns the media type of the Layer.
func (l *Layer) MediaType() (types.MediaType, error) {
	return l.desc.MediaType, nil
}

// Descriptor returns the original descriptor from an image manifest. See partial.Descriptor.
func (l *Layer) Descriptor() (*v1.Descriptor, error) {
	return &l.desc, nil
}
