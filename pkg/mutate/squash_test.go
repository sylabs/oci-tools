// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"testing"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sebdah/goldie/v2"
)

func TestShouldContinue(t *testing.T) {
	tests := []struct {
		name string
		path file.Path
		fn   filenode.FileNode
		want bool
	}{
		{
			"RegularFile",
			file.Path("test.txt"),
			filenode.FileNode{
				RealPath: file.Path("test.txt"),
				FileType: file.TypeRegular,
				Reference: &file.Reference{
					RealPath: file.Path("test.txt"),
				},
			},
			true,
		},
		{
			"SymlinkFile",
			file.Path("usr/bin/X11"),
			filenode.FileNode{
				RealPath: file.Path("usr/bin/X11"),
				FileType: file.TypeSymLink,
				LinkPath: ".",
				Reference: &file.Reference{
					RealPath: file.Path("usr/bin/X11"),
				},
			},
			true,
		},
		{
			"SecondPassOverSymlinkFile",
			file.Path("usr/bin/X11"),
			filenode.FileNode{
				RealPath: file.Path("usr/bin/X11"),
				FileType: file.TypeSymLink,
				LinkPath: ".",
				Reference: &file.Reference{
					RealPath: file.Path("usr/bin/X11"),
				},
			},
			false,
		},
	}

	cb := shouldContinue()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canContinue := cb(tt.path, tt.fn)
			if canContinue != tt.want {
				t.Errorf("should continue error - got: %v, want: %v", canContinue, tt.want)
			}
		})
	}
}

func TestShouldVisit(t *testing.T) {
	tests := []struct {
		name string
		path file.Path
		fn   filenode.FileNode
		want bool
	}{
		{
			"RegularFile",
			file.Path("test.txt"),
			filenode.FileNode{
				RealPath: file.Path("test.txt"),
				FileType: file.TypeRegular,
				Reference: &file.Reference{
					RealPath: file.Path("test.txt"),
				},
			},
			true,
		},
		{
			"SymlinkFile",
			file.Path("test.txt"),
			filenode.FileNode{
				RealPath: file.Path("test.txt"),
				FileType: file.TypeSymLink,
				LinkPath: file.Path("linked.txt"),
				Reference: &file.Reference{
					RealPath: file.Path("test.txt"),
				},
			},
			true,
		},
		{
			"SymlinkParentDir",
			file.Path("somefile.txt"),
			filenode.FileNode{
				RealPath: file.Path("someotherfile.txt"),
				FileType: file.TypeRegular,
				Reference: &file.Reference{
					RealPath: file.Path("someotherfile.txt"),
				},
			},
			false,
		},
		{
			"SymlinkDir",
			file.Path("/"),
			filenode.FileNode{
				RealPath:  file.Path("/"),
				FileType:  file.TypeDirectory,
				Reference: nil,
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canVisit := shouldVisit(tt.path, tt.fn)
			if canVisit != tt.want {
				t.Errorf("should visit error - got: %v, want: %v", canVisit, tt.want)
			}
		})
	}
}

func Benchmark_writeTAR(b *testing.B) {
	tests := []struct {
		name string
		base v1.Image
	}{
		{
			name: "DockerManifest",
			base: corpus.Image(b, "hello-world-docker-v2-manifest"),
		},
	}

	for _, tt := range tests {
		tt := tt

		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if err := writeTAR(tt.base, io.Discard); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func Test_writeTAR(t *testing.T) {
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

			if err := writeTAR(tt.base, &b); err != nil {
				t.Fatal(err)
			}

			g := goldie.New(t, goldie.WithTestNameForDir(true))
			g.Assert(t, tt.name, b.Bytes())
		})
	}
}

func Test_writeTAR_diff(t *testing.T) {
	tests := []struct {
		name      string
		base      v1.Image
		srcRootFS v1.Hash
	}{
		{
			name:      "DockerManifest",
			base:      corpus.Image(t, "hello-world-docker-v2-manifest"),
			srcRootFS: v1.Hash{Algorithm: "sha256", Hex: "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b bytes.Buffer

			if err := writeTAR(tt.base, &b); err != nil {
				t.Fatal(err)
			}

			l, err := tt.base.LayerByDigest(tt.srcRootFS)
			if err != nil {
				t.Fatal(err)
			}

			srcReader, err := l.Uncompressed()
			if err != nil {
				t.Fatal(err)
			}

			checkDiff(t, tar.NewReader(srcReader), tar.NewReader(bytes.NewReader(b.Bytes())))
		})
	}
}

func checkDiff(t *testing.T, srcReader *tar.Reader, sifTarReader *tar.Reader) {
	t.Helper()

	for {
		srcHdr, err := srcReader.Next()
		if errors.Is(err, io.EOF) {
			if _, err := sifTarReader.Next(); !errors.Is(err, io.EOF) {
				t.Errorf("expected EOF, got %v", err)
			}
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		cnvHdr, err := sifTarReader.Next()
		if err != nil {
			t.Fatal(err)
		}

		// Source header may contain bits outside of the TAR spec. Since the archive/tar package
		// will not write these (https://github.com/golang/go/issues/20150), use a mask to account
		// for the difference.
		srcHdr.Mode &= 0o7777
		if !reflect.DeepEqual(srcHdr, cnvHdr) {
			t.Errorf("Header mismatch: src: %+v | converted: %+v\n", srcHdr, cnvHdr)
		}

		srcFileData, err := io.ReadAll(srcReader)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Fatalf("err: %v", err)
		}
		cnvFileData, err := io.ReadAll(sifTarReader)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Fatalf("err: %v", err)
		}

		if !bytes.Equal(srcFileData, cnvFileData) {
			fmt.Printf("data mismatch: src: %x | converted: %x\n", srcFileData, cnvFileData)
		}
	}
}

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
		tt := tt

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
