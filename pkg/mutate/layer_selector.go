// Copyright 2024 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"errors"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// layerSelector is a list of selected layer indexs. Negative indexes are supported, for example
// an index of -1 would select the last layer in the image. If the underlying slice is nil, all
// layers are selected.
type layerSelector []int

// rangeLayerSelector returns a layerSelector that selects indicies from start up to end.
func rangeLayerSelector(start, end int) layerSelector {
	if start >= end {
		return layerSelector([]int{})
	}

	var s layerSelector
	if start < end {
		for i := start; i < end; i++ {
			s = append(s, i)
		}
	}
	return s
}

var errLayerIndexOutOfRange = errors.New("layer index out of range")

// layerSelected returns true if s indicates that layer i is selected in an image with n layers.
func (s layerSelector) indexSelected(i, n int) (bool, error) {
	if s == nil {
		return true, nil
	}

	for _, index := range s {
		if index < 0 {
			index += n
		}

		if index < 0 || n <= index {
			return false, errLayerIndexOutOfRange
		}

		if index == i {
			return true, nil
		}
	}

	return false, nil
}

// layersSelected returns the selected layers from im.
func (s layerSelector) layersSelected(im v1.Image) ([]v1.Layer, error) {
	ls, err := im.Layers()
	if err != nil {
		return nil, err
	}

	if s == nil {
		return ls, nil
	}

	var selected []v1.Layer

	for i, l := range ls {
		if ok, err := s.indexSelected(i, len(ls)); err != nil {
			return nil, err
		} else if ok {
			selected = append(selected, l)
		}
	}

	return selected, nil
}
