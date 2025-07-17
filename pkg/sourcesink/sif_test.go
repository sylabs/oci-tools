// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sourcesink

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sebdah/goldie/v2"
	"github.com/sylabs/oci-tools/pkg/ociplatform"
	"github.com/sylabs/oci-tools/test"
)

//nolint:gochecknoglobals
var corpus = test.NewCorpus(filepath.Join("..", "..", "test"))

func TestSIFFromPath(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		opts    []Option
		wantErr error
	}{
		{
			name: "Defaults",
			src:  corpus.SIF(t, "hello-world-docker-v2-manifest"),
		},
		{
			name:    "WithInstrumentationLogs",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts:    []Option{OptWithInstrumentationLogs(slog.Default())},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SIFFromPath(tt.src, tt.opts...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("SIFFromPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSIFGet(t *testing.T) {
	imgDigest := v1.Hash{Algorithm: "sha256", Hex: "432f982638b3aefab73cc58ab28f5c16e96fdb504e8c134fc58dff4bae8bf338"}
	idxDigest := v1.Hash{Algorithm: "sha256", Hex: "00e1ee7c898a2c393ea2fe7680938f8dcbe55e51fbf08032cf37326a677f92ed"}

	tests := []struct {
		name    string
		src     string
		opts    []GetOpt
		wantErr error
	}{
		{
			name:    "ImageDefaults",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts:    []GetOpt{},
			wantErr: nil,
		},
		{
			name:    "ImagePlatform",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts:    []GetOpt{GetWithPlatform(v1.Platform{OS: "Linux", Architecture: "arm64"})},
			wantErr: nil,
		},
		{
			name:    "ImageBadPlatform",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts:    []GetOpt{GetWithPlatform(v1.Platform{OS: "Linux", Architecture: "m68k"})},
			wantErr: ErrNoManifest,
		},
		{
			name:    "ImageDigest",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts:    []GetOpt{GetWithDigest(imgDigest)},
			wantErr: nil,
		},
		{
			name:    "ImageBadDigest",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts:    []GetOpt{GetWithDigest(v1.Hash{})},
			wantErr: ErrNoManifest,
		},
		{
			name:    "IndexDefaults",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest-list"),
			opts:    []GetOpt{},
			wantErr: nil,
		},
		{
			name:    "IndexPlatform",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest-list"),
			opts:    []GetOpt{GetWithPlatform(v1.Platform{OS: "Linux", Architecture: "arm64", Variant: "v8"})},
			wantErr: nil,
		},
		{
			name:    "IndexBadPlatform",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest-list"),
			opts:    []GetOpt{GetWithPlatform(v1.Platform{OS: "Linux", Architecture: "m68k"})},
			wantErr: ErrNoManifest,
		},
		{
			name:    "IndexDigest",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest-list"),
			opts:    []GetOpt{GetWithDigest(idxDigest)},
			wantErr: nil,
		},
		{
			name:    "IndexBadDigest",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest-list"),
			opts:    []GetOpt{GetWithDigest(v1.Hash{})},
			wantErr: ErrNoManifest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := SIFFromPath(tt.src)
			if err != nil {
				t.Fatalf("SIFFromPath() error = %v", err)
			}
			d, err := s.Get(t.Context(), tt.opts...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			mf, err := d.RawManifest()
			if err != nil {
				t.Fatalf("RawManifest() error = %v", err)
			}
			g := goldie.New(t, goldie.WithTestNameForDir(true))
			g.Assert(t, tt.name, mf)
		})
	}
}

func TestSIFDescriptorImage(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		wantPlatform v1.Platform
	}{
		{
			name:         "FromImage",
			src:          corpus.SIF(t, "hello-world-docker-v2-manifest"),
			wantPlatform: v1.Platform{OS: "Linux", Architecture: "arm64", Variant: "v8"},
		},
		{
			name:         "FromIndex",
			src:          corpus.SIF(t, "hello-world-docker-v2-manifest-list"),
			wantPlatform: *ociplatform.DefaultPlatform(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := SIFFromPath(tt.src)
			if err != nil {
				t.Fatalf("SIFFromPath() error = %v", err)
			}
			d, err := s.Get(t.Context())
			if err != nil {
				t.Fatalf(".Get() error = %v", err)
			}

			img, err := d.Image()
			if err != nil {
				t.Fatalf(".Image() error = %v", err)
			}

			if err := ociplatform.EnsureImageSatisfies(img, tt.wantPlatform); err != nil {
				t.Fatalf("Image does not satisfy expected platform %v", tt.wantPlatform)
			}
		})
	}
}

func TestSIFDescriptorImageIndex(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr error
	}{
		{
			name:    "FromImage",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest"),
			wantErr: ErrUnsupportedMediaType,
		},
		{
			name:    "FromIndex",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest-list"),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := SIFFromPath(tt.src)
			if err != nil {
				t.Fatalf("SIFFromPath() error = %v", err)
			}
			d, err := s.Get(t.Context())
			if err != nil {
				t.Fatalf(".Get() error = %v", err)
			}

			_, err = d.ImageIndex()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf(".ImageIndex() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSIFEmpty(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr error
	}{
		{
			name: "Defaults",
		},
		{
			name:    "WithInstrumentationLogs",
			opts:    []Option{OptWithInstrumentationLogs(slog.Default())},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.sif")
			_, err := SIFEmpty(path, 16, tt.opts...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("SIFEmpty() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSIFWrite(t *testing.T) {
	tests := []struct {
		name        string
		write       Writable
		descriptors int64
		opts        []WriteOpt
	}{
		{
			name:        "ImageDefaults",
			write:       corpus.Image(t, "hello-world-docker-v2-manifest"),
			descriptors: 4,
			opts:        []WriteOpt{},
		},
		{
			name:        "ImageWithReference",
			write:       corpus.Image(t, "hello-world-docker-v2-manifest"),
			descriptors: 4,
			opts:        []WriteOpt{WriteWithReference(name.MustParseReference("my:image", name.WithDefaultRegistry("")))},
		},
		{
			name:        "IndexDefaults",
			write:       corpus.ImageIndex(t, "hello-world-docker-v2-manifest-list"),
			descriptors: 32,
			opts:        []WriteOpt{},
		},
		{
			name:        "IndexWithReference",
			write:       corpus.ImageIndex(t, "hello-world-docker-v2-manifest-list"),
			descriptors: 32,
			opts:        []WriteOpt{WriteWithReference(name.MustParseReference("my:image", name.WithDefaultRegistry("")))},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.sif")

			s, err := SIFEmpty(path, tt.descriptors)
			if err != nil {
				t.Fatalf("SIFEmpty() error = %v", err)
			}
			err = s.Write(t.Context(), tt.write, tt.opts...)
			if err != nil {
				t.Fatalf(".Write() error = %v", err)
			}

			index, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("while reading test.sif: %v", err)
			}
			g := goldie.New(t, goldie.WithTestNameForDir(true))
			g.Assert(t, tt.name, index)
		})
	}
}

func TestSIFBlob(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		opts    []GetOpt
		wantErr error
	}{
		{
			name: "ImageConfig",
			src:  corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts: []GetOpt{GetWithDigest(
				v1.Hash{Algorithm: "sha256", Hex: "46331d942d6350436f64e614d75725f6de3bb5c63e266e236e04389820a234c4"})},
			wantErr: nil,
		},
		{
			name: "ImageLayer",
			src:  corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts: []GetOpt{GetWithDigest(
				v1.Hash{Algorithm: "sha256", Hex: "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01"})},
			wantErr: nil,
		},
		{
			name:    "ErrNoDigest",
			src:     corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts:    []GetOpt{},
			wantErr: errSIFBlobNoDigest,
		},
		{
			name: "ErrPlatform",
			src:  corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts: []GetOpt{
				GetWithDigest(v1.Hash{Algorithm: "sha256", Hex: "7050e35b49f5e348c4809f5eff95842962cb813f32062d3bbdd35c750dd7d01"}),
				GetWithPlatform(*ociplatform.DefaultPlatform()),
			},
			wantErr: errSIFBlobPlatform,
		},
		{
			name: "ErrReference",
			src:  corpus.SIF(t, "hello-world-docker-v2-manifest"),
			opts: []GetOpt{
				GetWithDigest(v1.Hash{Algorithm: "sha256", Hex: "7050e35b49f5e348c4809f5eff95842962cb813f32062d3bbdd35c750dd7d01"}),
				GetWithReference(name.MustParseReference("test")),
			},
			wantErr: errSIFBlobReference,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := SIFFromPath(tt.src)
			if err != nil {
				t.Fatalf("OCIFromPath() error = %v", err)
			}
			rc, err := s.Blob(t.Context(), tt.opts...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("ReadAll error: %v", err)
			}

			g := goldie.New(t, goldie.WithTestNameForDir(true))
			g.Assert(t, tt.name, b)
		})
	}
}
