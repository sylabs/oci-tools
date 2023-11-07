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

var _ partial.CompressedLayer = (*layer)(nil)

type layer struct {
	f    *fileImage
	desc v1.Descriptor
}

// Digest returns the Hash of the compressed layer.
func (l *layer) Digest() (v1.Hash, error) {
	return l.desc.Digest, nil
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
func (l *layer) Compressed() (io.ReadCloser, error) {
	return l.f.Blob(l.desc.Digest)
}

// Size returns the compressed size of the Layer.
func (l *layer) Size() (int64, error) {
	return l.desc.Size, nil
}

// MediaType returns the media type of the Layer.
func (l *layer) MediaType() (types.MediaType, error) {
	return l.desc.MediaType, nil
}

// Descriptor returns the original descriptor from an image manifest. See partial.Descriptor.
func (l *layer) Descriptor() (*v1.Descriptor, error) {
	return &l.desc, nil
}
