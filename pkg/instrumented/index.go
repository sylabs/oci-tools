// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package instrumented

import (
	"log/slog"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type wrappedIndex struct {
	inner v1.ImageIndex
	log   *slog.Logger
}

// Index returns a wrapped ImageIndex that outputs instrumentation to log.
func Index(ii v1.ImageIndex, log *slog.Logger) (v1.ImageIndex, error) {
	h, err := ii.Digest()
	if err != nil {
		return nil, err
	}

	return &wrappedIndex{
		inner: ii,
		log:   log.With(slog.String("index", h.Hex)),
	}, nil
}

// MediaType of this image's manifest.
func (ii *wrappedIndex) MediaType() (types.MediaType, error) {
	defer func(t time.Time) {
		ii.log.Info("MediaType()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return ii.inner.MediaType()
}

// Digest returns the sha256 of this image's manifest.
func (ii *wrappedIndex) Digest() (v1.Hash, error) {
	defer func(t time.Time) {
		ii.log.Info("Digest()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return ii.inner.Digest()
}

// Size returns the size of the manifest.
func (ii *wrappedIndex) Size() (int64, error) {
	defer func(t time.Time) {
		ii.log.Info("Size()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return ii.inner.Size()
}

// IndexManifest returns this image index's manifest object.
func (ii *wrappedIndex) IndexManifest() (*v1.IndexManifest, error) {
	defer func(t time.Time) {
		ii.log.Info("IndexManifest()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return ii.inner.IndexManifest()
}

// RawManifest returns the serialized bytes of IndexManifest().
func (ii *wrappedIndex) RawManifest() ([]byte, error) {
	defer func(t time.Time) {
		ii.log.Info("RawManifest()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	return ii.inner.RawManifest()
}

// Image returns a v1.Image that this ImageIndex references.
func (ii *wrappedIndex) Image(d v1.Hash) (v1.Image, error) {
	defer func(t time.Time) {
		ii.log.Info("Image()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	img, err := ii.inner.Image(d)
	if err != nil {
		return nil, err
	}

	return Image(img, ii.log)
}

// ImageIndex returns a v1.ImageIndex that this ImageIndex references.
func (ii *wrappedIndex) ImageIndex(d v1.Hash) (v1.ImageIndex, error) {
	defer func(t time.Time) {
		ii.log.Info("ImageIndex()", slog.Duration("dur", time.Since(t)))
	}(time.Now())

	idx, err := ii.inner.ImageIndex(d)
	if err != nil {
		return nil, err
	}

	return Index(idx, ii.log)
}
