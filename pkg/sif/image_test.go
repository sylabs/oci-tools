// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif_test

import (
	"reflect"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/validate"
	"github.com/sylabs/oci-tools/pkg/sif"
)

// imageIndexFromPath returns an ImageIndex for the test to use, populated from the OCI Image
// Layout with the specified path in the corpus.
func imageIndexFromPath(t *testing.T, path string) v1.ImageIndex {
	t.Helper()

	ii, err := sif.ImageIndexFromFileImage(fileImageFromPath(t, path))
	if err != nil {
		t.Fatal(err)
	}

	return ii
}

func Test_imageIndex_Image(t *testing.T) {
	imageDigest := v1.Hash{
		Algorithm: "sha256",
		Hex:       "432f982638b3aefab73cc58ab28f5c16e96fdb504e8c134fc58dff4bae8bf338",
	}
	imageDescriptor := &v1.Descriptor{
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Size:      525,
		Digest:    imageDigest,
		Platform: &v1.Platform{
			Architecture: "arm64",
			OS:           "linux",
			Variant:      "v8",
		},
	}

	tests := []struct {
		name           string
		ii             v1.ImageIndex
		hash           v1.Hash
		wantErr        bool
		wantDescriptor *v1.Descriptor
	}{
		{
			name:           "DockerManifest",
			ii:             imageIndexFromPath(t, "hello-world-docker-v2-manifest"),
			hash:           imageDigest,
			wantDescriptor: imageDescriptor,
		},
		{
			name:           "DockerManifestList",
			ii:             imageIndexFromPath(t, "hello-world-docker-v2-manifest-list"),
			hash:           imageDigest,
			wantDescriptor: imageDescriptor,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := tt.ii.Image(tt.hash)
			if err != nil {
				t.Fatal(err)
			}

			if err := validate.Image(img); err != nil {
				t.Error(err)
			}

			if d, err := partial.Descriptor(img); err != nil {
				t.Error(err)
			} else if got, want := d, tt.wantDescriptor; !reflect.DeepEqual(got, want) {
				t.Errorf("got descriptor %+v, want %+v", got, want)
			}
		})
	}
}
