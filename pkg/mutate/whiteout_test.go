// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"maps"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func Test_scanAUFSOpaque(t *testing.T) {
	tests := []struct {
		name               string
		layer              v1.Layer
		expectOpaque       map[string]bool
		expectFileWhiteout bool
	}{
		// HelloWorld layer contains no opaque markers
		{
			name: "HelloWorldTar",
			layer: testLayer(t, "hello-world-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01",
			}),
			expectOpaque:       map[string]bool{},
			expectFileWhiteout: false,
		},
		// AUFS layer contains a single opaque marker on dir
		//        [drwxr-xr-x]  .
		//			├── [drwxr-xr-x]  dir
		//			│   └── [-rw-r--r--]  .wh..wh..opq
		//			└── [-rw-r--r--]  .wh.file
		{
			name: "AUFSTar",
			layer: testLayer(t, "aufs-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "da55812559dec81445c289c3832cee4a2f725b15aeb258791640185c3126b2bf",
			}),
			expectOpaque: map[string]bool{
				"dir": true,
			},
			expectFileWhiteout: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc, err := tt.layer.Uncompressed()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { rc.Close() })

			opaque, fileWhiteout, err := scanAUFSWhiteouts(rc)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !maps.Equal(tt.expectOpaque, opaque) {
				t.Errorf("opaque directories - expected: %v, got: %v", tt.expectOpaque, opaque)
			}
			if fileWhiteout != tt.expectFileWhiteout {
				t.Errorf("file whiteout(s) - expected: %v, got: %v", tt.expectFileWhiteout, fileWhiteout)
			}
		})
	}
}
