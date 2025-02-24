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
				Size:      444,
				Digest: v1.Hash{
					Algorithm: "sha256",
					Hex:       "61a28aa82cdc48531d7dc366298507a4e896cebce1e01ff391626edd968f6d58",
				},
			},
		},
		{
			name: "DockerManifestList",
			f:    fileImageFromPath(t, "hello-world-docker-v2-manifest-list"),
			wantDescriptor: &v1.Descriptor{
				MediaType: "application/vnd.oci.image.index.v1+json",
				Size:      323,
				Digest: v1.Hash{
					Algorithm: "sha256",
					Hex:       "18102125ef60453edd3543a30fd87469b3703999c8be7b58de67575f09ea74b2",
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

func Test_OCIFileImage_Index(t *testing.T) {
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

	idxRef := name.MustParseReference("myindex:v1", name.WithDefaultRegistry(""))
	idx, err := random.Index(64, 1, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}
	if err := ofi.AppendIndex(idx, sif.OptAppendReference(idxRef)); err != nil {
		t.Fatal(err)
	}
	idx2, err := random.Index(64, 1, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}
	if err := ofi.AppendIndex(idx2); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		matcher   match.Matcher
		wantIndex v1.ImageIndex
		wantErr   error
	}{
		{
			name:      "MatchingRef",
			matcher:   match.Name(idxRef.Name()),
			wantIndex: idx,
			wantErr:   nil,
		},
		{
			name:      "NotIndex",
			matcher:   match.Name(imgRef.Name()),
			wantIndex: nil,
			wantErr:   sif.ErrNoMatch,
		},
		{
			name:      "NonMatchingRef",
			matcher:   match.Name("not-present:latest"),
			wantIndex: nil,
			wantErr:   sif.ErrNoMatch,
		},
		{
			name:      "MultipleMatches",
			matcher:   match.MediaTypes(string(types.OCIImageIndex)),
			wantIndex: nil,
			wantErr:   sif.ErrMultipleMatches,
		},
		{
			name:      "NilMatcher",
			matcher:   nil,
			wantIndex: nil,
			wantErr:   sif.ErrMultipleMatches,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, err := ofi.Index(tt.matcher)

			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Fatalf("got error %v, want %v", got, want)
			}

			if tt.wantIndex == nil {
				return
			}

			gotDigest, err := gotIndex.Digest()
			if err != nil {
				t.Fatal(err)
			}
			wantDigest, err := tt.wantIndex.Digest()
			if err != nil {
				t.Fatal(err)
			}
			if gotDigest != wantDigest {
				t.Errorf("Expected index with digest %q, got %q", wantDigest, gotDigest)
			}
		})
	}
}
