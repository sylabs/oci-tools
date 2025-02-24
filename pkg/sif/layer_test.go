// Copyright 2023-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif_test

import (
	"reflect"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sylabs/oci-tools/pkg/sif"
)

// layerFromPath returns a Layer for the test to use, populated from the OCI Image with the
// specified path/digests in the corpus.
func layerFromPath(t *testing.T, path string, imageDigest, layerDigest string) v1.Layer {
	t.Helper()

	ii := imageIndexFromPath(t, path)

	imageHash, err := v1.NewHash(imageDigest)
	if err != nil {
		t.Fatal(err)
	}

	img, err := ii.Image(imageHash)
	if err != nil {
		t.Fatal(err)
	}

	layerHash, err := v1.NewHash(layerDigest)
	if err != nil {
		t.Fatal(err)
	}

	l, err := img.LayerByDigest(layerHash)
	if err != nil {
		t.Fatal(err)
	}

	return l
}

func TestLayer_Descriptor(t *testing.T) {
	tests := []struct {
		name           string
		l              v1.Layer
		wantDescriptor *v1.Descriptor
	}{
		{
			name: "DockerManifest",
			l: layerFromPath(t, "hello-world-docker-v2-manifest",
				"sha256:432f982638b3aefab73cc58ab28f5c16e96fdb504e8c134fc58dff4bae8bf338",
				"sha256:7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01",
			),
			wantDescriptor: &v1.Descriptor{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      3208,
				Digest: v1.Hash{
					Algorithm: "sha256",
					Hex:       "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if l, ok := tt.l.(*sif.Layer); !ok {
				t.Errorf("unexpected layer type: %T", tt.l)
			} else if d, err := l.Descriptor(); err != nil {
				t.Error(err)
			} else if got, want := d, tt.wantDescriptor; !reflect.DeepEqual(got, want) {
				t.Errorf("got descriptor %+v, want %+v", got, want)
			}
		})
	}
}

func TestLayer_Offset(t *testing.T) {
	tests := []struct {
		name       string
		l          v1.Layer
		wantOffset int64
	}{
		{
			name: "DockerManifest",
			l: layerFromPath(t, "hello-world-docker-v2-manifest",
				"sha256:432f982638b3aefab73cc58ab28f5c16e96fdb504e8c134fc58dff4bae8bf338",
				"sha256:7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01",
			),
			wantOffset: 6436,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if l, ok := tt.l.(*sif.Layer); !ok {
				t.Errorf("unexpected layer type: %T", tt.l)
			} else if d, err := l.Offset(); err != nil {
				t.Error(err)
			} else if got, want := d, tt.wantOffset; got != want {
				t.Errorf("got offset %v, want %v", got, want)
			}
		})
	}
}
