// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package instrumented

import (
	"log/slog"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type wrappedImage struct {
	inner v1.Image
	log   *slog.Logger
}

// Image returns a wrapped Image that outputs instrumentation to log.
func Image(img v1.Image, log *slog.Logger) (v1.Image, error) {
	h, err := img.Digest()
	if err != nil {
		return nil, err
	}

	return &wrappedImage{
		inner: img,
		log:   log.With(slog.String("image", h.Hex)),
	}, nil
}

// Descriptor returns a Descriptor for the image manifest.
func (img *wrappedImage) Descriptor() (*v1.Descriptor, error) {
	defer func(t time.Time) {
		img.log.Info("Descriptor()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return partial.Descriptor(img.inner)
}

// MediaType of this image's manifest.
func (img *wrappedImage) MediaType() (types.MediaType, error) {
	defer func(t time.Time) {
		img.log.Info("MediaType()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return img.inner.MediaType()
}

// Size returns the size of the manifest.
func (img *wrappedImage) Size() (int64, error) {
	defer func(t time.Time) {
		img.log.Info("Size()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return img.inner.Size()
}

// Digest returns the sha256 of this image's manifest.
func (img *wrappedImage) Digest() (v1.Hash, error) {
	defer func(t time.Time) {
		img.log.Info("Digest()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return img.inner.Digest()
}

// Manifest returns this image's Manifest object.
func (img *wrappedImage) Manifest() (*v1.Manifest, error) {
	defer func(t time.Time) {
		img.log.Info("Manifest()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return img.inner.Manifest()
}

// RawManifest returns the serialized bytes of Manifest().
func (img *wrappedImage) RawManifest() ([]byte, error) {
	defer func(t time.Time) {
		img.log.Info("RawManifest()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return img.inner.RawManifest()
}

// ConfigName returns the hash of the image's config file, also known as the Image ID.
func (img *wrappedImage) ConfigName() (v1.Hash, error) {
	defer func(t time.Time) {
		img.log.Info("ConfigName()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return img.inner.ConfigName()
}

// ConfigFile returns this image's config file.
func (img *wrappedImage) ConfigFile() (*v1.ConfigFile, error) {
	defer func(t time.Time) {
		img.log.Info("ConfigFile()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return img.inner.ConfigFile()
}

// RawConfigFile returns the serialized bytes of ConfigFile().
func (img *wrappedImage) RawConfigFile() ([]byte, error) {
	defer func(t time.Time) {
		img.log.Info("RawConfigFile()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return img.inner.RawConfigFile()
}

// Layers returns the ordered collection of filesystem layers that comprise this image.
func (img *wrappedImage) Layers() ([]v1.Layer, error) {
	defer func(t time.Time) {
		img.log.Info("Layers()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	ls, err := img.inner.Layers()
	if err != nil {
		return nil, err
	}

	for i, l := range ls {
		l, err := Layer(l, img.log)
		if err != nil {
			return nil, err
		}

		ls[i] = l
	}

	return ls, nil
}

// LayerByDigest returns a Layer for interacting with a particular layer of the image, looking it
// up by "digest" (the compressed hash).
func (img *wrappedImage) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	defer func(t time.Time) {
		img.log.Info("LayerByDigest()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	l, err := img.inner.LayerByDigest(h)
	if err != nil {
		return nil, err
	}

	return Layer(l, img.log)
}

// LayerByDiffID is an analog to LayerByDigest, looking up by "diff id" (the uncompressed hash).
func (img *wrappedImage) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	defer func(t time.Time) {
		img.log.Info("LayerByDiffID()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	l, err := img.inner.LayerByDiffID(h)
	if err != nil {
		return nil, err
	}

	return Layer(l, img.log)
}
