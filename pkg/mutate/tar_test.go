// Copyright 2024 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"errors"
	"io"
	"os/exec"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sebdah/goldie/v2"
)

func Test_TarFromSquashfsLayer(t *testing.T) {
	tests := []struct {
		name     string
		layer    v1.Layer
		opts     []TarConverterOpt
		diffArgs []string
	}{
		// SquashFS layer contains whiteouts in OverlayFS format. Should be
		// converted to AUFS form when noConvertWhiteout = false.
		//
		//    Original (OverlayFS)
		//
		//          [drwxr-xr-x]  .
		//          ├── [drwxr-xr-x]  dir		(xattr trusted.overlay.opaque="y")
		//          └── [crw-r--r--]  file		(0,0 char device)
		//
		//
		//    Converted (AUFS)
		//      All regular files.
		//
		//        [drwxr-xr-x]  .
		//          ├── [drwxr-xr-x]  dir
		//          │   └── [-rw-r--r--]  .wh..wh..opq
		//          └── [-rw-r--r--]  .wh.file
		//
		{
			name: "OverlayFSBlob",
			layer: testLayer(t, "overlayfs-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "2addb7e8ed33f5f080813d437f455a2ae0c6a3cd41f978eaa05fc776d4f7a887",
			}),
		},
		{
			name: "OverlayFSBlob_SkipWhiteoutConversion",
			layer: testLayer(t, "overlayfs-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "2addb7e8ed33f5f080813d437f455a2ae0c6a3cd41f978eaa05fc776d4f7a887",
			}),
			opts: []TarConverterOpt{OptTarSkipWhiteoutConversion(true)},
		},
		{
			name: "OverlayFSBlobTempDir",
			layer: testLayer(t, "overlayfs-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "2addb7e8ed33f5f080813d437f455a2ae0c6a3cd41f978eaa05fc776d4f7a887",
			}),
			opts: []TarConverterOpt{OptTarTempDir(t.TempDir())},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := exec.LookPath("sqfs2tar"); errors.Is(err, exec.ErrNotFound) {
				t.Skip(err)
			}

			opener, err := TarFromSquashfsLayer(tt.layer, tt.opts...)
			if err != nil {
				t.Fatal(err)
			}

			rc, err := opener()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { rc.Close() })

			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}

			g := goldie.New(t, goldie.WithTestNameForDir(true))
			g.Assert(t, tt.name, data)
		})
	}
}
