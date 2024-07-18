// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"bytes"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sebdah/goldie/v2"
)

func TestSquash(t *testing.T) {
	tests := []struct {
		name string
		base v1.Image
		s    layerSelector
	}{
		{
			name: "RootDirEntry",
			base: corpus.Image(t, "root-dir-entry"),
		},
		{
			name: "DockerManifest",
			base: corpus.Image(t, "hello-world-docker-v2-manifest"),
		},
		{
			name: "WhiteoutExplicitFile",
			base: corpus.Image(t, "whiteout-explicit-file"),
		},
		{
			name: "WhiteoutExplicitDir",
			base: corpus.Image(t, "whiteout-explicit-dir"),
		},
		{
			name: "WhiteoutOpaque",
			base: corpus.Image(t, "whiteout-opaque"),
		},
		{
			name: "WhiteoutOpaqueEnd",
			base: corpus.Image(t, "whiteout-opaque-end"),
		},
		{
			name: "HardLink1",
			base: corpus.Image(t, "hard-link-1"),
		},
		{
			name: "HardLink2",
			base: corpus.Image(t, "hard-link-2"),
		},
		{
			name: "HardLinkDelete1",
			base: corpus.Image(t, "hard-link-delete-1"),
		},
		{
			name: "HardLinkDelete2",
			base: corpus.Image(t, "hard-link-delete-2"),
		},
		{
			name: "HardLinkDelete3",
			base: corpus.Image(t, "hard-link-delete-3"),
		},
		{
			name: "HardLinkDelete4",
			base: corpus.Image(t, "hard-link-delete-4"),
		},
		{
			name: "HardLinkDeleteXattr",
			base: corpus.Image(t, "hard-link-delete-xattr"),
		},
		{
			name: "LayerSelector",
			base: corpus.Image(t, "hard-link-delete-4"),
			s:    rangeLayerSelector(0, 2),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b bytes.Buffer

			if err := squash(tt.base, tt.s, &b); err != nil {
				t.Fatal(err)
			}

			g := goldie.New(t,
				goldie.WithTestNameForDir(true),
				goldie.WithSubTestNameForDir(true),
			)

			g.Assert(t, "layer", b.Bytes())
		})
	}
}
