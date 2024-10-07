// Copyright 2024 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	match "github.com/google/go-containerregistry/pkg/v1/match"
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

			ofi, err := sif.FromFileImage(fi)
			if err != nil {
				t.Fatal(err)
			}

			ii, err := ofi.RootIndex()
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

//nolint:dupl
func TestAppendImage(t *testing.T) {
	r := rand.NewSource(randomSeed)
	newImage, err := random.Image(64, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		base string
		opts []sif.AppendOpt
	}{
		{
			name: "Default",
			base: "hello-world-docker-v2-manifest",
			opts: []sif.AppendOpt{},
		},
		{
			name: "WithReference",
			base: "hello-world-docker-v2-manifest",
			opts: []sif.AppendOpt{
				sif.OptAppendReference(name.MustParseReference("myimage:v1", name.WithDefaultRegistry(""))),
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

			ofi, err := sif.FromFileImage(fi)
			if err != nil {
				t.Fatal(err)
			}

			if err := ofi.AppendImage(newImage, tt.opts...); err != nil {
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

//nolint:dupl
func TestAppendIndex(t *testing.T) {
	r := rand.NewSource(randomSeed)
	newIndex, err := random.Index(64, 1, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		base string
		opts []sif.AppendOpt
	}{
		{
			name: "Default",
			base: "hello-world-docker-v2-manifest",
			opts: []sif.AppendOpt{},
		},
		{
			name: "WithReference",
			base: "hello-world-docker-v2-manifest",
			opts: []sif.AppendOpt{
				sif.OptAppendReference(name.MustParseReference("myindex:v1", name.WithDefaultRegistry(""))),
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

			ofi, err := sif.FromFileImage(fi)
			if err != nil {
				t.Fatal(err)
			}

			if err := ofi.AppendIndex(newIndex, tt.opts...); err != nil {
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

func TestAppendMultiple(t *testing.T) {
	r := rand.NewSource(randomSeed)
	image1, err := random.Image(64, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}
	image2, err := random.Image(64, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}
	sifPath := corpus.SIF(t, "hello-world-docker-v2-manifest", sif.OptWriteWithSpareDescriptorCapacity(8))
	fi, err := ssif.LoadContainerFromPath(sifPath)
	if err != nil {
		t.Fatal(err)
	}
	ofi, err := sif.FromFileImage(fi)
	if err != nil {
		t.Fatal(err)
	}
	referenceOpt := sif.OptAppendReference(name.MustParseReference("myimage:v1", name.WithDefaultRegistry("")))
	if err := ofi.AppendImage(image1, referenceOpt); err != nil {
		t.Fatal(err)
	}
	if err := ofi.AppendImage(image2, referenceOpt); err != nil {
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
	g.Assert(t, "image", b)
}

func TestRemoveBlob(t *testing.T) {
	validDigest, err := v1.NewHash("sha256:7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01")
	if err != nil {
		t.Fatal(err)
	}

	otherDigest, err := v1.NewHash("sha256:e66fc843f1291ede94f0ecb3dbd8d277d4b05a8a4ceba1e211365dae9adb17da")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		base    string
		digest  v1.Hash
		wantErr bool
	}{
		{
			name:    "Valid",
			base:    "hello-world-docker-v2-manifest",
			digest:  validDigest,
			wantErr: false,
		},
		{
			name:    "NotFound",
			base:    "hello-world-docker-v2-manifest",
			digest:  otherDigest,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sifPath := corpus.SIF(t, tt.base, sif.OptWriteWithSpareDescriptorCapacity(8))
			fi, err := ssif.LoadContainerFromPath(sifPath)
			if err != nil {
				t.Fatal(err)
			}

			ofi, err := sif.FromFileImage(fi)
			if err != nil {
				t.Fatal(err)
			}

			err = ofi.RemoveBlob(tt.digest)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, but nil returned")
				}
				return
			}
			if err != nil {
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

func TestRemoveManifests(t *testing.T) {
	tests := []struct {
		name    string
		matcher match.Matcher
		base    string
	}{
		{
			name:    "Valid",
			base:    "hello-world-docker-v2-manifest-list",
			matcher: match.Platforms(v1.Platform{OS: "linux", Architecture: "ppc64le"}),
		},
		{
			name:    "NoMatch",
			base:    "hello-world-docker-v2-manifest-list",
			matcher: match.Platforms(v1.Platform{OS: "linux", Architecture: "m68k"}),
		},
		{
			name:    "NilMatcher",
			base:    "hello-world-docker-v2-manifest-list",
			matcher: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sifPath := corpus.SIF(t, tt.base, sif.OptWriteWithSpareDescriptorCapacity(8))
			fi, err := ssif.LoadContainerFromPath(sifPath)
			if err != nil {
				t.Fatal(err)
			}

			ofi, err := sif.FromFileImage(fi)
			if err != nil {
				t.Fatal(err)
			}

			if err := ofi.RemoveManifests(tt.matcher); err != nil {
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

//nolint:dupl
func TestReplace(t *testing.T) {
	r := rand.NewSource(randomSeed)
	newImage, err := random.Image(64, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}
	newIndex, err := random.Index(64, 1, 1, random.WithSource(r))
	if err != nil {
		t.Fatal(err)
	}

	replaceImage := func(ofi *sif.OCIFileImage, m match.Matcher) error { return ofi.ReplaceImage(newImage, m) }
	replaceIndex := func(ofi *sif.OCIFileImage, m match.Matcher) error { return ofi.ReplaceIndex(newIndex, m) }
	tests := []struct {
		name        string
		base        string
		replacement func(ofi *sif.OCIFileImage, m match.Matcher) error
		matcher     match.Matcher
	}{
		{
			name:        "ReplaceImageManifest",
			base:        "hello-world-docker-v2-manifest",
			replacement: replaceImage,
			matcher:     match.Platforms(v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"}),
		},
		{
			name:        "ReplaceImageManifestList",
			base:        "hello-world-docker-v2-manifest-list",
			replacement: replaceImage,
			matcher:     match.Platforms(v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"}),
		},
		{
			name:        "ReplaceImageNoMatch",
			base:        "hello-world-docker-v2-manifest",
			replacement: replaceImage,
			matcher:     match.Platforms(v1.Platform{OS: "linux", Architecture: "m68k"}),
		},
		{
			name:        "ReplaceImageNilMatcher",
			base:        "hello-world-docker-v2-manifest",
			replacement: replaceImage,
			matcher:     nil,
		},
		{
			name:        "ReplaceIndexManifest",
			base:        "hello-world-docker-v2-manifest",
			replacement: replaceIndex,
			matcher:     match.Platforms(v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"}),
		},
		{
			name:        "ReplaceIndexManifestList",
			base:        "hello-world-docker-v2-manifest-list",
			replacement: replaceIndex,
			matcher:     match.Platforms(v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"}),
		},
		{
			name:        "ReplaceIndexNoMatch",
			base:        "hello-world-docker-v2-manifest",
			replacement: replaceIndex,
			matcher:     match.Platforms(v1.Platform{OS: "linux", Architecture: "m68k"}),
		},
		{
			name:        "ReplaceIndexNilMatcher",
			base:        "hello-world-docker-v2-manifest",
			replacement: replaceIndex,
			matcher:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sifPath := corpus.SIF(t, tt.base, sif.OptWriteWithSpareDescriptorCapacity(8))
			fi, err := ssif.LoadContainerFromPath(sifPath)
			if err != nil {
				t.Fatal(err)
			}

			ofi, err := sif.FromFileImage(fi)
			if err != nil {
				t.Fatal(err)
			}

			if err := tt.replacement(ofi, tt.matcher); err != nil {
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
