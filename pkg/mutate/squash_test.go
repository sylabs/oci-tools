// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"bytes"
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
			var b bytes.Buffer

			if err := squash(tt.base, &b); err != nil {
				t.Fatal(err)
			}

			g := goldie.New(t,
				goldie.WithTestNameForDir(true),
				goldie.WithSubTestNameForDir(true),
			)

			g.Assert(t, "layer", b.Bytes())
		})
	}
}
