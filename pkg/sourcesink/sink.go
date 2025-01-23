// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sourcesink

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
)

// Writable can represent a v1.Image or v1.ImageIndex. Consumers of a Writable
// will cast to one of these types.
type Writable interface {
	RawManifest() ([]byte, error)
}

// Sink implements methods to write images and indexes to a specific type of
// storage, and location.
type Sink interface {
	Write(ctx context.Context, w Writable, opts ...WriteOpt) error
}

// writeOpts holds options that should apply across to a single Write operation
// against a sink.
type writeOpts struct {
	reference name.Reference
}

// WriteOpt sets an option that applies to a single Write operation against a
// sink.
type WriteOpt func(*writeOpts) error

// WriteWithReference will set a reference for the image or index written, at
// the destination.
func WriteWithReference(r name.Reference) WriteOpt {
	return func(o *writeOpts) error {
		o.reference = r
		return nil
	}
}
