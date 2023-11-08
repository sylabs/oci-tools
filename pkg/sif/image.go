// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"errors"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

var _ partial.CompressedImageCore = (*image)(nil)

type image struct {
	f           *fileImage
	desc        *v1.Descriptor
	rawManifest []byte
}

// MediaType of this image's manifest.
func (im *image) MediaType() (types.MediaType, error) {
	return im.desc.MediaType, nil
}

// RawConfigFile returns the serialized bytes of ConfigFile().
func (im *image) RawConfigFile() ([]byte, error) {
	manifest, err := im.Manifest()
	if err != nil {
		return nil, err
	}

	return im.f.Bytes(manifest.Config.Digest)
}

// Manifest returns this image's Manifest object.
func (im *image) Manifest() (*v1.Manifest, error) {
	return partial.Manifest(im)
}

// RawManifest returns the serialized bytes of Manifest().
func (im *image) RawManifest() ([]byte, error) {
	return im.rawManifest, nil
}

// Descriptor returns the original descriptor from an index manifest. See partial.Descriptor.
func (im *image) Descriptor() (*v1.Descriptor, error) {
	return im.desc, nil
}

var errLayerNotFoundInImage = errors.New("layer not found in image")

// LayerByDigest returns a Layer for interacting with a particular layer of the image, looking it
// up by "digest" (the compressed hash).
func (im *image) LayerByDigest(h v1.Hash) (partial.CompressedLayer, error) {
	manifest, err := im.Manifest()
	if err != nil {
		return nil, err
	}

	if h == manifest.Config.Digest {
		return &layer{
			f:    im.f,
			desc: manifest.Config,
		}, nil
	}

	for _, desc := range manifest.Layers {
		if h == desc.Digest {
			return &layer{
				f:    im.f,
				desc: desc,
			}, nil
		}
	}

	return nil, fmt.Errorf("%w: %v", errLayerNotFoundInImage, h)
}
