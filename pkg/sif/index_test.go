// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/validate"
	"github.com/sylabs/oci-tools/pkg/sif"
	ssif "github.com/sylabs/sif/v2/pkg/sif"
)

func imageIndexFromPath(t *testing.T, path string) v1.ImageIndex {
	t.Helper()

	f, err := ssif.LoadContainerFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.UnloadContainer() })

	ii, err := sif.ImageIndexFromFileImage(f)
	if err != nil {
		t.Fatal(err)
	}

	return ii
}

func TestFile_ImageIndex(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "DockerManifest",
			path: corpus.SIF(t, "hello-world-docker-v2-manifest"),
		},
		{
			name: "DockerManifestList",
			path: corpus.SIF(t, "hello-world-docker-v2-manifest-list"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ii := imageIndexFromPath(t, tt.path)

			if err := validate.Index(ii); err != nil {
				t.Fatal(err)
			}
		})
	}
}
