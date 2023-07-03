// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Use a fixed digest, so that this is repeatable.
//
//nolint:gochecknoglobals, lll
var helloWorldRef = name.MustParseReference("hello-world@sha256:00e1ee7c898a2c393ea2fe7680938f8dcbe55e51fbf08032cf37326a677f92ed")

func generateImages(path string) error {
	platform := v1.Platform{
		Architecture: "arm64",
		OS:           "linux",
		Variant:      "v8",
	}

	images := []struct {
		source      name.Reference
		opts        []remote.Option
		destination string
	}{
		{
			source:      helloWorldRef,
			destination: filepath.Join(path, "hello-world-docker-v2-manifest"),
			opts: []remote.Option{
				remote.WithPlatform(platform),
			},
		},
	}

	for _, im := range images {
		img, err := remote.Image(im.source, im.opts...)
		if err != nil {
			return err
		}

		desc, err := partial.Descriptor(img)
		if err != nil {
			return err
		}

		ii := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
			Add:        img,
			Descriptor: *desc,
		})

		if _, err := layout.Write(im.destination, ii); err != nil {
			return err
		}
	}

	return nil
}

func generateIndexes(path string) error {
	images := []struct {
		source      name.Reference
		opts        []remote.Option
		destination string
	}{
		{
			source:      helloWorldRef,
			destination: filepath.Join(path, "hello-world-docker-v2-manifest-list"),
		},
	}

	for _, im := range images {
		ii, err := remote.Index(im.source, im.opts...)
		if err != nil {
			return err
		}

		if _, err := layout.Write(im.destination, ii); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	path := "."
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	if err := generateImages(path); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if err := generateIndexes(path); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
