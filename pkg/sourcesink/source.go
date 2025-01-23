// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sourcesink

import (
	"context"
	"errors"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Descriptor wraps a v1.Descriptor, providing methods to access the image or
// index to which it pertains, and the associated manifest.
type Descriptor interface {
	// RawManifest returns the manifest of the image or index described by this
	// descriptor.
	RawManifest() ([]byte, error)
	// MediaType returns the types.MediaType of this descriptor.
	MediaType() types.MediaType
	// Image returns a v1.Image directly if the descriptor is associated with an
	// OCI image, or an image for the local platform if the descriptor is
	// associated with an OCI ImageIndex.
	Image() (v1.Image, error)
	// ImageIndex returns a v1.ImageIndex if the descriptor is associated with
	// an OCI ImageIndex.
	ImageIndex() (v1.ImageIndex, error)
}

// Source implements methods to read images and indexes from a specific type of
// storage, and location.
type Source interface {
	// Get will find an image or index at the source that matches the
	// requirements specified by opts. The image or index is returned as a
	// Descriptor.
	Get(ctx context.Context, opts ...GetOpt) (Descriptor, error)
	// Blob will return an io.ReadCloser that provides the content of the blob
	// specified via opts.
	Blob(ctx context.Context, opts ...GetOpt) (io.ReadCloser, error)
}

// getOpts holds options that should apply across to a single Get operation
// against a source.
type getOpts struct {
	platform  *v1.Platform
	digest    *v1.Hash
	reference name.Reference
}

// GetOpt sets an option that applies to a single Get operation against a
// source.
type GetOpt func(*getOpts) error

// GetPlatform overrides the platform when choosing an image.
func GetWithPlatform(p v1.Platform) GetOpt {
	return func(o *getOpts) error {
		o.platform = &p
		return nil
	}
}

// GetDigest sets the digest of the image or index to retrieve.
func GetWithDigest(d v1.Hash) GetOpt {
	return func(o *getOpts) error {
		o.digest = &d
		return nil
	}
}

// GetReference sets the name.ref of the image or index to retrieve.
func GetWithReference(r name.Reference) GetOpt {
	return func(o *getOpts) error {
		o.reference = r
		return nil
	}
}

var (
	// ErrNoManifest is returned when no manifests that satisfy provided
	// criteria are found in a source.
	ErrNoManifest = errors.New("no matching image / index found")
	// ErrMultipleManifests is returned when more than one manifest that
	// satisfies provided criteria is found in a source.
	ErrMultipleManifests = errors.New("multiple matching images / indices found")
	// ErrUnsupportedMediaType is returned when an operation is attempted
	// against an OCI media type that does not support it.
	ErrUnsupportedMediaType = errors.New("unsupported media type")
)
