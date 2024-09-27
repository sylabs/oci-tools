// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif_test

import (
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
	"github.com/sylabs/oci-tools/pkg/sif"
	ssif "github.com/sylabs/sif/v2/pkg/sif"
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

func Test_OCIFileImage_Image(t *testing.T) {
	tmpDir := t.TempDir()
	sifPath := filepath.Join(tmpDir, "test.sif")
	if err := sif.Write(sifPath, empty.Index, sif.OptWriteWithSpareDescriptorCapacity(16)); err != nil {
		t.Fatal(err)
	}
	fi, err := ssif.LoadContainerFromPath(sifPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fi.UnloadContainer() })
	ofi, err := sif.FromFileImage(fi)
	if err != nil {
		t.Fatal(err)
	}

	imgRef := name.MustParseReference("myimage:v1", name.WithDefaultRegistry(""))
	r := rand.NewSource(randomSeed)
	img, err := random.Image(64, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}
	if err := ofi.AppendImage(img, sif.OptAppendReference(imgRef)); err != nil {
		t.Fatal(err)
	}
	img2, err := random.Image(64, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}
	if err := ofi.AppendImage(img2); err != nil {
		t.Fatal(err)
	}

	idxRef := name.MustParseReference("myindex:v1", name.WithDefaultRegistry(""))
	idx, err := random.Index(64, 1, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}
	if err := ofi.AppendIndex(idx, sif.OptAppendReference(idxRef)); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		matcher   match.Matcher
		wantImage v1.Image
		wantErr   bool
	}{
		{
			name:      "MatchingRef",
			matcher:   match.Name(imgRef.Name()),
			wantImage: img,
			wantErr:   false,
		},
		{
			name:      "NotImage",
			matcher:   match.Name(idxRef.Name()),
			wantImage: nil,
			wantErr:   true,
		},
		{
			name:      "NonMatchingRef",
			matcher:   match.Name("not-present:latest"),
			wantImage: nil,
			wantErr:   true,
		},
		{
			name:      "MultipleMatches",
			matcher:   match.MediaTypes(string(types.DockerManifestSchema2)),
			wantImage: nil,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotImg, err := ofi.Image(tt.matcher)

			if err != nil && !tt.wantErr {
				t.Errorf("Unexpected error: %v", err)
			}
			if err == nil && tt.wantErr {
				t.Errorf("Error expected, but nil returned.")
			}

			if tt.wantImage == nil {
				return
			}

			gotDigest, err := gotImg.Digest()
			if err != nil {
				t.Fatal(err)
			}
			wantDigest, err := tt.wantImage.Digest()
			if err != nil {
				t.Fatal(err)
			}
			if gotDigest != wantDigest {
				t.Errorf("Expected image with digest %q, got %q", wantDigest, gotDigest)
			}
		})
	}
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

			if d, err := img.(withDescriptor).Descriptor(); err != nil {
				t.Error(err)
			} else if got, want := d, tt.wantDescriptor; !reflect.DeepEqual(got, want) {
				t.Errorf("got descriptor %+v, want %+v", got, want)
			}
		})
	}
}
