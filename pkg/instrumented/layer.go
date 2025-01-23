// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package instrumented

import (
	"io"
	"log/slog"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type wrappedLayer struct {
	inner v1.Layer
	log   *slog.Logger
}

// Layer returns a wrapped Layer that outputs instrumentation to log.
func Layer(l v1.Layer, log *slog.Logger) (v1.Layer, error) {
	h, err := l.Digest()
	if err != nil {
		return nil, err
	}

	return &wrappedLayer{
		inner: l,
		log:   log.With(slog.String("layer", h.Hex)),
	}, nil
}

// Digest returns the Hash of the compressed layer.
func (l *wrappedLayer) Digest() (v1.Hash, error) {
	defer func(t time.Time) {
		l.log.Info("Digest()",
			slog.Duration("dur", time.Since(t)),
		)
	}(time.Now())

	return l.inner.Digest()
}

// DiffID implements v1.Layer.
func (l *wrappedLayer) DiffID() (v1.Hash, error) {
	defer func(t time.Time) {
		l.log.Info("DiffID()",
			slog.Duration("dur", time.Since(t)),
		)
	}(time.Now())

	return l.inner.DiffID()
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
func (l *wrappedLayer) Compressed() (io.ReadCloser, error) {
	defer func(t time.Time) {
		l.log.Info("Compressed()",
			slog.Duration("dur", time.Since(t)),
		)
	}(time.Now())

	rc, err := l.inner.Compressed()
	if err != nil {
		return nil, err
	}

	return readCloser(rc, l.log.With(slog.Bool("compressed", true))), nil
}

// Uncompressed implements v1.Layer.
func (l *wrappedLayer) Uncompressed() (io.ReadCloser, error) {
	defer func(t time.Time) {
		l.log.Info("Uncompressed()",
			slog.Duration("dur", time.Since(t)),
		)
	}(time.Now())

	rc, err := l.inner.Uncompressed()
	if err != nil {
		return nil, err
	}

	return readCloser(rc, l.log.With(slog.Bool("compressed", false))), nil
}

// Size returns the compressed size of the Layer.
func (l *wrappedLayer) Size() (int64, error) {
	defer func(t time.Time) {
		l.log.Info("Size()",
			slog.Duration("dur", time.Since(t)),
		)
	}(time.Now())

	return l.inner.Size()
}

// MediaType returns the media type of the Layer.
func (l *wrappedLayer) MediaType() (types.MediaType, error) {
	defer func(t time.Time) {
		l.log.Info("MediaType()",
			slog.Duration("dur", time.Since(t)),
		)
	}(time.Now())

	return l.inner.MediaType()
}

// Descriptor returns a Descriptor for the layer.
func (l *wrappedLayer) Descriptor() (*v1.Descriptor, error) {
	defer func(t time.Time) {
		l.log.Info("Descriptor()",
			slog.Duration("dur", time.Since(t)),
		)
	}(time.Now())

	return partial.Descriptor(l.inner)
}
