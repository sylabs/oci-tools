// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Squash replaces the layers in the base image with a single, squashed layer.
func Squash(base v1.Image) (v1.Image, error) {
	opener := func() (io.ReadCloser, error) {
		return mutate.Extract(base), nil
	}

	l, err := tarball.LayerFromOpener(opener)
	if err != nil {
		return nil, err
	}

	return Apply(base, ReplaceLayers(l))
}
