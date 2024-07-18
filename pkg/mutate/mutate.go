// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"errors"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type Mutation func(*image) error

var errInvalidLayerIndex = errors.New("invalid layer index")

// SetLayer sets the layer at index i to l.
func SetLayer(i int, l v1.Layer) Mutation {
	return func(img *image) error {
		if i >= len(img.overrides) {
			return errInvalidLayerIndex
		}

		img.overrides[i] = l

		return nil
	}
}

// ReplaceLayers replaces all layers in the image with l.
func ReplaceLayers(l v1.Layer) Mutation {
	return func(img *image) error {
		img.overrides = []v1.Layer{l}
		return nil
	}
}

// replaceSelectedLayers replaces selected layers in the image with l.
func replaceSelectedLayers(s layerSelector, l v1.Layer) Mutation {
	return func(img *image) error {
		var found bool
		var overrides []v1.Layer

		// Iterate over the current layers, replacing matching layers with rl.
		for i, override := range img.overrides {
			selected, err := s.indexSelected(i, len(img.overrides))
			if err != nil {
				return err
			}

			if !selected {
				overrides = append(overrides, override)
			} else if !found {
				overrides = append(overrides, l)
				found = true
			}
		}

		img.overrides = overrides
		return nil
	}
}

// SetHistory replaces the history in an image with the specified entry.
func SetHistory(history v1.History) Mutation {
	return func(img *image) error {
		img.history = &history
		return nil
	}
}

// SetConfig replaces the config with the specified raw content of type t.
func SetConfig(configFile any, configType types.MediaType) Mutation {
	return func(img *image) error {
		img.configFileOverride = configFile
		img.configTypeOverride = configType
		return nil
	}
}

// Apply performs the specified mutation(s) to a base image, returning the resulting image.
func Apply(base v1.Image, ms ...Mutation) (v1.Image, error) {
	if len(ms) == 0 {
		return base, nil
	}

	layers, err := base.Layers()
	if err != nil {
		return nil, err
	}

	img := image{
		base:      base,
		overrides: make([]v1.Layer, len(layers)),
	}

	for _, m := range ms {
		if err := m(&img); err != nil {
			return nil, err
		}
	}

	return &img, nil
}
