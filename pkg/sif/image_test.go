// Copyright 2023-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif_test

import (
	"errors"
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
		wantErr   error
	}{
		{
			name:      "MatchingRef",
			matcher:   match.Name(imgRef.Name()),
			wantImage: img,
			wantErr:   nil,
		},
		{
			name:      "NotImage",
			matcher:   match.Name(idxRef.Name()),
			wantImage: nil,
			wantErr:   sif.ErrNoMatch,
		},
		{
			name:      "NonMatchingRef",
			matcher:   match.Name("not-present:latest"),
			wantImage: nil,
			wantErr:   sif.ErrNoMatch,
		},
		{
			name:      "MultipleMatches",
			matcher:   match.MediaTypes(string(types.DockerManifestSchema2)),
			wantImage: nil,
			wantErr:   sif.ErrMultipleMatches,
		},
		{
			name:      "NilMatcher",
			matcher:   nil,
			wantImage: nil,
			wantErr:   sif.ErrMultipleMatches,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotImg, err := ofi.Image(tt.matcher)

			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Fatalf("got error %v, want %v", got, want)
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

			if wd, ok := img.(withDescriptor); !ok {
				t.Error("withDescriptor interface not satisfied")
			} else if d, err := wd.Descriptor(); err != nil {
				t.Error(err)
			} else if got, want := d, tt.wantDescriptor; !reflect.DeepEqual(got, want) {
				t.Errorf("got descriptor %+v, want %+v", got, want)
			}
		})
	}
}

func Test_imageIndex_Index(t *testing.T) {
	indexDigest := v1.Hash{
		Algorithm: "sha256",
		Hex:       "00e1ee7c898a2c393ea2fe7680938f8dcbe55e51fbf08032cf37326a677f92ed",
	}
	indexDescriptor := &v1.Descriptor{
		MediaType: "application/vnd.docker.distribution.manifest.list.v2+json",
		Size:      2069,
		Digest:    indexDigest,
	}

	tests := []struct {
		name           string
		ii             v1.ImageIndex
		hash           v1.Hash
		wantErr        bool
		wantDescriptor *v1.Descriptor
	}{
		{
			name:           "DockerManifestList",
			ii:             imageIndexFromPath(t, "hello-world-docker-v2-manifest-list"),
			hash:           indexDigest,
			wantDescriptor: indexDescriptor,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ii, err := tt.ii.ImageIndex(tt.hash)
			if err != nil {
				t.Fatal(err)
			}

			if err := validate.Index(ii); err != nil {
				t.Error(err)
			}

			if wd, ok := ii.(withDescriptor); !ok {
				t.Error("withDescriptor interface not satisfied")
			} else if d, err := wd.Descriptor(); err != nil {
				t.Error(err)
			} else if got, want := d, tt.wantDescriptor; !reflect.DeepEqual(got, want) {
				t.Errorf("got descriptor %+v, want %+v", got, want)
			}
		})
	}
}
