// Copyright 2024 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif_test

import (
	"math/rand"
	"os"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	v1mutate "github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sebdah/goldie/v2"
	"github.com/sylabs/oci-tools/pkg/mutate"
	"github.com/sylabs/oci-tools/pkg/sif"
	ssif "github.com/sylabs/sif/v2/pkg/sif"
)

const randomSeed = 1719306160

//nolint:gocognit
func TestUpdate(t *testing.T) {
	r := rand.NewSource(randomSeed)

	tests := []struct {
		name    string
		base    string
		updater func(*testing.T, v1.ImageIndex) v1.ImageIndex
		opts    []sif.UpdateOpt
	}{
		{
			name: "AddLayer",
			base: "hello-world-docker-v2-manifest",
			updater: func(t *testing.T, ii v1.ImageIndex) v1.ImageIndex {
				t.Helper()
				ih, err := v1.NewHash("sha256:432f982638b3aefab73cc58ab28f5c16e96fdb504e8c134fc58dff4bae8bf338")
				if err != nil {
					t.Fatal(err)
				}
				im, err := ii.Image(ih)
				if err != nil {
					t.Fatal(err)
				}
				l, err := random.Layer(64, types.DockerLayer, random.WithSource(r))
				if err != nil {
					t.Fatal(err)
				}
				im, err = v1mutate.AppendLayers(im, l)
				if err != nil {
					t.Fatal(err)
				}
				return v1mutate.AppendManifests(empty.Index, v1mutate.IndexAddendum{Add: im})
			},
		},
		{
			name: "ReplaceLayers", // Replaces many layers with a single layer
			base: "many-layers",
			updater: func(t *testing.T, ii v1.ImageIndex) v1.ImageIndex {
				t.Helper()
				ih, err := v1.NewHash("sha256:7c000de5bc837f29d1c9a5e76bba79922d860e5c0f448df3b6fc38431a067c9a")
				if err != nil {
					t.Fatal(err)
				}
				im, err := ii.Image(ih)
				if err != nil {
					t.Fatal(err)
				}
				l, err := random.Layer(64, types.DockerLayer, random.WithSource(r))
				if err != nil {
					t.Fatal(err)
				}
				im, err = mutate.Apply(im, mutate.ReplaceLayers(l))
				if err != nil {
					t.Fatal(err)
				}
				return v1mutate.AppendManifests(empty.Index, v1mutate.IndexAddendum{Add: im})
			},
		},
		{
			name: "AddImage",
			base: "hello-world-docker-v2-manifest",
			updater: func(t *testing.T, ii v1.ImageIndex) v1.ImageIndex {
				t.Helper()
				im, err := random.Image(64, 1, random.WithSource(r))
				if err != nil {
					t.Fatal(err)
				}
				if err != nil {
					t.Fatal(err)
				}
				return v1mutate.AppendManifests(ii, v1mutate.IndexAddendum{Add: im})
			},
		},
		{
			name: "AddImageIndex",
			base: "hello-world-docker-v2-manifest",
			updater: func(t *testing.T, ii v1.ImageIndex) v1.ImageIndex {
				t.Helper()
				addIdx, err := random.Index(64, 1, 1, random.WithSource(r))
				if err != nil {
					t.Fatal(err)
				}
				if err != nil {
					t.Fatal(err)
				}
				return v1mutate.AppendManifests(ii, v1mutate.IndexAddendum{Add: addIdx})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sifPath := corpus.SIF(t, tt.base, sif.OptWriteWithSpareDescriptorCapacity(8))
			fi, err := ssif.LoadContainerFromPath(sifPath)
			if err != nil {
				t.Fatal(err)
			}
			ii, err := sif.ImageIndexFromFileImage(fi)
			if err != nil {
				t.Fatal(err)
			}

			ii = tt.updater(t, ii)

			if err := sif.Update(fi, ii, tt.opts...); err != nil {
				t.Fatal(err)
			}

			if err := fi.UnloadContainer(); err != nil {
				t.Fatal(err)
			}

			b, err := os.ReadFile(sifPath)
			if err != nil {
				t.Fatal(err)
			}

			g := goldie.New(t,
				goldie.WithTestNameForDir(true),
			)

			g.Assert(t, tt.name, b)
		})
	}
}
