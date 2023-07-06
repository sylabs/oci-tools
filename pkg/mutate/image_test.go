// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// testHash returns a new SHA256 hash.
func testHash(tb testing.TB, hash string) v1.Hash {
	tb.Helper()

	h, err := v1.NewHash("sha256:" + hash)
	if err != nil {
		tb.Fatal(err)
	}
	return h
}

func Test_image_populate(t *testing.T) { //nolint:gocognit
	img := corpus.Image(t, "hello-world-docker-v2-manifest")

	tests := []struct {
		name            string
		img             *image
		wantMediaType   types.MediaType
		wantSize        int64
		wantDigest      v1.Hash
		wantConfigName  v1.Hash
		wantLayers      int
		wantLayerDigest v1.Hash
		wantLayerDiffID v1.Hash
	}{
		{
			name: "Base",
			img: &image{
				base:      img,
				overrides: make([]v1.Layer, 1),
			},
			wantMediaType:   types.DockerManifestSchema2,
			wantSize:        424,
			wantDigest:      testHash(t, "73b965ea5a7262bd50e3a4d0b8a9fb4b262aae7d40ea92a2fd892955199e20bd"),
			wantConfigName:  testHash(t, "93ad81f8071afb1a00ef481a6034c0bf59a18bc2e1fba8bceceecb0acf2bddc3"),
			wantLayers:      1,
			wantLayerDigest: testHash(t, "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01"),
			wantLayerDiffID: testHash(t, "efb53921da3394806160641b72a2cbd34ca1a9a8345ac670a85a04ad3d0e3507"),
		},
		{
			name: "Layer",
			img: &image{
				base: img,
				overrides: []v1.Layer{
					static.NewLayer([]byte("foobar"), types.DockerLayer),
				},
			},
			wantMediaType:   types.DockerManifestSchema2,
			wantSize:        421,
			wantDigest:      testHash(t, "2f4916476e18acf5d828aac21832af735e5f10285989e9c11fa54cf128253bd4"),
			wantConfigName:  testHash(t, "5bc8c5c79bcaec30fe6388bdef34dae553665ac248cee2e9d69c30dc97e64faf"),
			wantLayers:      1,
			wantLayerDigest: testHash(t, "c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2"),
			wantLayerDiffID: testHash(t, "c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2"),
		},
		{
			name: "History",
			img: &image{
				base:      img,
				overrides: make([]v1.Layer, 1),
				history: &v1.History{
					Author:    "Author",
					Created:   v1.Time{Time: time.Date(2023, 5, 2, 2, 25, 50, 0, time.UTC)},
					CreatedBy: "CreatedBy",
					Comment:   "Comment",
				},
			},
			wantMediaType:   types.DockerManifestSchema2,
			wantSize:        424,
			wantDigest:      testHash(t, "b4aaf4456c40b0ea0c3dc6f55f6e04dd209503d5acbb447c3678a0d046376bf1"),
			wantConfigName:  testHash(t, "9b95bea653c4830c69c83b161842570c048b8a53eebd691367d97e07403106e3"),
			wantLayers:      1,
			wantLayerDigest: testHash(t, "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01"),
			wantLayerDiffID: testHash(t, "efb53921da3394806160641b72a2cbd34ca1a9a8345ac670a85a04ad3d0e3507"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := tt.img.MediaType()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := mt, tt.wantMediaType; got != want {
				t.Errorf("got media type %v, want %v", got, want)
			}

			size, err := tt.img.Size()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := size, tt.wantSize; got != want {
				t.Errorf("got size %v, want %v", got, want)
			}

			digest, err := tt.img.Digest()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := digest, tt.wantDigest; got != want {
				t.Errorf("got digest %v, want %v", got, want)
			}

			manifest, err := tt.img.Manifest()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := manifest, tt.img.manifest; !reflect.DeepEqual(got, want) {
				t.Errorf("got manifest %+v, want %+v", got, want)
			}

			rawManifest, err := tt.img.RawManifest()
			if err != nil {
				t.Fatal(err)
			}
			var m v1.Manifest
			if err := json.Unmarshal(rawManifest, &m); err != nil {
				t.Fatal(err)
			}
			if got, want := m, *tt.img.manifest; !reflect.DeepEqual(got, want) {
				t.Errorf("got manifest %+v, want %+v", got, want)
			}

			configName, err := tt.img.ConfigName()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := configName, tt.wantConfigName; got != want {
				t.Errorf("got config name %v, want %v", got, want)
			}

			configFile, err := tt.img.ConfigFile()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := configFile, tt.img.configFile; !reflect.DeepEqual(got, want) {
				t.Errorf("got config file %+v, want %+v", got, want)
			}

			rawConfigFile, err := tt.img.RawConfigFile()
			if err != nil {
				t.Fatal(err)
			}
			var c v1.ConfigFile
			if err := json.Unmarshal(rawConfigFile, &c); err != nil {
				t.Fatal(err)
			}
			if got, want := c, *tt.img.configFile; !reflect.DeepEqual(got, want) {
				t.Errorf("got config file %+v, want %+v", got, want)
			}

			layers, err := tt.img.Layers()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(layers), tt.wantLayers; got != want {
				t.Errorf("got %v layers, want %v", got, want)
			}

			if _, err := tt.img.LayerByDigest(tt.wantLayerDigest); err != nil {
				t.Errorf("layer not found with digest %v", tt.wantLayerDigest)
			}

			if _, err := tt.img.LayerByDiffID(tt.wantLayerDiffID); err != nil {
				t.Errorf("layer not found with diff id %v", tt.wantLayerDiffID)
			}
		})
	}
}
