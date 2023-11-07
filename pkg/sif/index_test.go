// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif_test

import (
	"reflect"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/validate"
	"github.com/sylabs/oci-tools/pkg/sif"
	ssif "github.com/sylabs/sif/v2/pkg/sif"
)

type withDescriptor interface {
	Descriptor() (*v1.Descriptor, error)
}

// fileImageFromPath returns a temporary FileImage for the test to use, populated from the OCI
// Image Layout with the specified path in the corpus. The FileImage is automatically unloaded when
// the test and all its subtests complete.
func fileImageFromPath(t *testing.T, path string) *ssif.FileImage {
	t.Helper()

	f, err := ssif.LoadContainerFromPath(corpus.SIF(t, path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.UnloadContainer() })

	return f
}

func TestImageIndexFromFileImage(t *testing.T) {
	tests := []struct {
		name           string
		f              *ssif.FileImage
		wantDescriptor *v1.Descriptor
	}{
		{
			name: "DockerManifest",
			f:    fileImageFromPath(t, "hello-world-docker-v2-manifest"),
			wantDescriptor: &v1.Descriptor{
				MediaType: "application/vnd.oci.image.index.v1+json",
				Size:      314,
				Digest: v1.Hash{
					Algorithm: "sha256",
					Hex:       "a2ca8d2eb29d4b32cabd3f2ca67c14c8ae178b93c3000da3ec63faca49a688e4",
				},
			},
		},
		{
			name: "DockerManifestList",
			f:    fileImageFromPath(t, "hello-world-docker-v2-manifest-list"),
			wantDescriptor: &v1.Descriptor{
				MediaType: "application/vnd.oci.image.index.v1+json",
				Size:      2069,
				Digest: v1.Hash{
					Algorithm: "sha256",
					Hex:       "00e1ee7c898a2c393ea2fe7680938f8dcbe55e51fbf08032cf37326a677f92ed",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ii, err := sif.ImageIndexFromFileImage(tt.f)
			if err != nil {
				t.Fatal(err)
			}

			if err := validate.Index(ii); err != nil {
				t.Error(err)
			}

			if d, err := ii.(withDescriptor).Descriptor(); err != nil {
				t.Error(err)
			} else if got, want := d, tt.wantDescriptor; !reflect.DeepEqual(got, want) {
				t.Errorf("got descriptor %+v, want %+v", got, want)
			}
		})
	}
}
