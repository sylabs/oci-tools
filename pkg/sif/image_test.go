// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestFile_Image(t *testing.T) {
	tests := []struct {
		name string
		path string
		hash v1.Hash
	}{
		{
			name: "DockerManifest",
			path: corpus.SIF(t, "hello-world-docker-v2-manifest"),
			hash: v1.Hash{
				Algorithm: "sha256",
				Hex:       "432f982638b3aefab73cc58ab28f5c16e96fdb504e8c134fc58dff4bae8bf338",
			},
		},
		{
			name: "DockerManifestList",
			path: corpus.SIF(t, "hello-world-docker-v2-manifest-list"),
			hash: v1.Hash{
				Algorithm: "sha256",
				Hex:       "432f982638b3aefab73cc58ab28f5c16e96fdb504e8c134fc58dff4bae8bf338",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ii := imageIndexFromPath(t, tt.path)

			img, err := ii.Image(tt.hash)
			if err != nil {
				t.Fatal(err)
			}

			if err := validate.Image(img); err != nil {
				t.Fatal(err)
			}
		})
	}
}
