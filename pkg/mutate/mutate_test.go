// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sebdah/goldie/v2"
	"github.com/sylabs/oci-tools/test"
)

//nolint:gochecknoglobals
var corpus = test.NewCorpus(filepath.Join("..", "..", "test"))

func TestApply(t *testing.T) {
	img := corpus.Image(t, "hello-world-docker-v2-manifest")

	tests := []struct {
		name string
		base v1.Image
		ms   []Mutation
	}{
		{
			name: "NoMutation",
			base: img,
		},
		{
			name: "SetLayer",
			base: img,
			ms: []Mutation{
				SetLayer(0, static.NewLayer([]byte("foobar"), types.DockerLayer)),
			},
		},
		{
			name: "ReplaceLayers",
			base: img,
			ms: []Mutation{
				ReplaceLayers(static.NewLayer([]byte("foobar"), types.DockerLayer)),
			},
		},
		{
			name: "SetHistory",
			base: img,
			ms: []Mutation{
				SetHistory(v1.History{
					Author:    "Author",
					Created:   v1.Time{Time: time.Date(2023, 5, 2, 2, 25, 50, 0, time.UTC)},
					CreatedBy: "CreatedBy",
					Comment:   "Comment",
				}),
			},
		},
		{
			name: "SetConfigCustom",
			base: img,
			ms: []Mutation{
				SetConfig(struct{ Foo string }{"Bar"}, "application/vnd.sylabs.container.image.v1+json"),
			},
		},
		{
			name: "SetConfigDocker",
			base: img,
			ms: []Mutation{
				SetConfig(&v1.ConfigFile{Author: "Author"}, types.DockerConfigJSON),
			},
		},
		{
			name: "SetConfigEmpty",
			base: img,
			ms: []Mutation{
				SetConfig(struct{}{}, "application/vnd.oci.empty.v1+json"),
			},
		},
		{
			name: "SetArtifactType",
			base: img,
			ms: []Mutation{
				SetConfig(struct{}{}, "application/vnd.oci.empty.v1+json"),
				SetArtifactType("application/vnd.sylabs.container"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := Apply(tt.base, tt.ms...)
			if err != nil {
				t.Fatal(err)
			}

			g := goldie.New(t,
				goldie.WithTestNameForDir(true),
				goldie.WithSubTestNameForDir(true),
			)

			config, err := img.RawConfigFile()
			if err != nil {
				t.Fatal(err)
			}

			g.Assert(t, "config", config)

			manifest, err := img.RawManifest()
			if err != nil {
				t.Fatal(err)
			}

			g.Assert(t, "manifest", manifest)
		})
	}
}
