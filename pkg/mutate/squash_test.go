// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sebdah/goldie/v2"
)

func TestSquash(t *testing.T) {
	tests := []struct {
		name string
		base v1.Image
	}{
		{
			name: "DockerManifest",
			base: corpus.Image(t, "hello-world-docker-v2-manifest"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := Squash(tt.base)
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
