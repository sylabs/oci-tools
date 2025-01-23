// Copyright 2023-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif_test

import (
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	ggcrempty "github.com/google/go-containerregistry/pkg/v1/empty"
	ggcrmutate "github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/sebdah/goldie/v2"
	"github.com/sylabs/oci-tools/pkg/sif"
	"github.com/sylabs/oci-tools/test"
)

//nolint:gochecknoglobals
var corpus = test.NewCorpus(filepath.Join("..", "..", "test"))

func TestWrite(t *testing.T) {
	tests := []struct {
		name string
		ii   v1.ImageIndex
		opts []sif.WriteOpt
	}{
		{
			name: "DockerManifest",
			ii: ggcrmutate.AppendManifests(
				ggcrempty.Index,
				ggcrmutate.IndexAddendum{Add: corpus.Image(t, "hello-world-docker-v2-manifest")}),
		},
		{
			name: "DockerManifestList",
			ii:   corpus.ImageIndex(t, "hello-world-docker-v2-manifest-list"),
		},
		{
			name: "ManyLayers",
			ii: ggcrmutate.AppendManifests(
				ggcrempty.Index,
				ggcrmutate.IndexAddendum{Add: corpus.Image(t, "many-layers")}),
		},
		{
			name: "SpareDescriptor",
			ii: ggcrmutate.AppendManifests(
				ggcrempty.Index,
				ggcrmutate.IndexAddendum{Add: corpus.Image(t, "hello-world-docker-v2-manifest")}),
			opts: []sif.WriteOpt{
				sif.OptWriteWithSpareDescriptorCapacity(1),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "image.sif")

			if err := sif.Write(path, tt.ii, tt.opts...); err != nil {
				t.Fatal(err)
			}

			b, err := os.ReadFile(path)
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
