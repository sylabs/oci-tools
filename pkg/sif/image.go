// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"bytes"
	"errors"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

var _ v1.Image = (*image)(nil)

type image struct {
	f           *fileImage
	desc        *v1.Descriptor
	rawManifest []byte
}

// Layers returns the ordered collection of filesystem layers that comprise this image. The order
// of the list is oldest/base layer first, and most-recent/top layer last.
func (im *image) Layers() ([]v1.Layer, error) {
	m, err := im.Manifest()
	if err != nil {
		return nil, err
	}

	ls := make([]v1.Layer, len(m.Layers))
	for i, d := range m.Layers {
		l, err := im.LayerByDigest(d.Digest)
		if err != nil {
			return nil, err
		}

		ls[i] = l
	}

	return ls, nil
}

// MediaType of this image's manifest.
func (im *image) MediaType() (types.MediaType, error) {
	return im.desc.MediaType, nil
}

// Size returns the size of the manifest.
func (im *image) Size() (int64, error) {
	return im.desc.Size, nil
}

// ConfigName returns the hash of the image's config file, also known as the Image ID.
func (im *image) ConfigName() (v1.Hash, error) {
	b, err := im.RawConfigFile()
	if err != nil {
		return v1.Hash{}, err
	}

	h, _, err := v1.SHA256(bytes.NewReader(b))
	return h, err
}

// ConfigFile returns this image's config file.
func (im *image) ConfigFile() (*v1.ConfigFile, error) {
	b, err := im.RawConfigFile()
	if err != nil {
		return nil, err
	}

	return v1.ParseConfigFile(bytes.NewReader(b))
}

// RawConfigFile returns the serialized bytes of ConfigFile().
func (im *image) RawConfigFile() ([]byte, error) {
	manifest, err := im.Manifest()
	if err != nil {
		return nil, err
	}

	return im.f.Bytes(manifest.Config.Digest)
}

// Digest returns the sha256 of this image's manifest.
func (im *image) Digest() (v1.Hash, error) {
	h, _, err := v1.SHA256(bytes.NewReader(im.rawManifest))
	return h, err
}

// Manifest returns this image's Manifest object.
func (im *image) Manifest() (*v1.Manifest, error) {
	return v1.ParseManifest(bytes.NewReader(im.rawManifest))
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
func (im *image) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	manifest, err := im.Manifest()
	if err != nil {
		return nil, err
	}

	for _, desc := range manifest.Layers {
		if h == desc.Digest {
			return &Layer{
				f:    im.f,
				desc: desc,
			}, nil
		}
	}

	return nil, fmt.Errorf("%w: %v", errLayerNotFoundInImage, h)
}

// LayerByDiffID is an analog to LayerByDigest, looking up by "diff id" (the uncompressed hash).
func (im *image) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	h, err := partial.DiffIDToBlob(im, h)
	if err != nil {
		return nil, err
	}

	return im.LayerByDigest(h)
}
