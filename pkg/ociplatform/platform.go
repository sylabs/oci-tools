// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package ociplatform

import (
	"errors"
	"fmt"

	"github.com/containerd/platforms"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// specsPlatform converts a ggcr v1.Platform to a specs-go v1.Platform.
func specsPlatform(p ggcrv1.Platform) specsv1.Platform {
	return specsv1.Platform{
		Architecture: p.Architecture,
		OS:           p.OS,
		OSVersion:    p.OSVersion,
		OSFeatures:   p.OSFeatures,
		Variant:      p.Variant,
	}
}

// ggcrPlatform converts a specs-go v1.Platform to a ggcr v1.Platform.
func ggcrPlatform(p specsv1.Platform) ggcrv1.Platform {
	return ggcrv1.Platform{
		Architecture: p.Architecture,
		OS:           p.OS,
		OSVersion:    p.OSVersion,
		OSFeatures:   p.OSFeatures,
		Variant:      p.Variant,
	}
}

// DefaultPlatform returns the local machine's platform as a ggcr v1.Platform.
func DefaultPlatform() *ggcrv1.Platform {
	dp := ggcrPlatform(platforms.DefaultSpec())
	return &dp
}

// ImageSatisfies returns true if img satisfies platform, using the
// containerd/platforms matcher, which applies normalization rules. If an image
// has no platform, then it is considered to satisfy any platform specification.
func ImageSatisfies(img ggcrv1.Image, platform ggcrv1.Platform) (bool, error) {
	cf, err := img.ConfigFile()
	if err != nil {
		return false, err
	}

	cfp := cf.Platform()
	if cfp == nil {
		return true, nil
	}

	imgPlatform := specsPlatform(*cfp)
	targetPlatform := specsPlatform(platform)
	m := platforms.NewMatcher(targetPlatform)
	return m.Match(imgPlatform), nil
}

var ErrPlatformNotSatisfied = errors.New("image does not satisfy platform")

// EnsureImageSatisfies returns an error if img does not satisfy platform, using
// the containerd/platforms matcher, which applies normalization rules.
func EnsureImageSatisfies(img ggcrv1.Image, platform ggcrv1.Platform) error {
	ok, err := ImageSatisfies(img, platform)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrPlatformNotSatisfied, platform.String())
	}
	return nil
}

// DescriptorSatisfies returns true if desc satisfies platform, using the
// containerd/platforms matcher, which applies normalization rules. If a
// descriptor has no platform, then it is considered to satisfy any platform
// specification.
func DescriptorSatisfies(desc ggcrv1.Descriptor, platform ggcrv1.Platform) bool {
	if desc.Platform == nil {
		return true
	}

	descPlatform := specsPlatform(*desc.Platform)
	targetPlatform := specsPlatform(platform)
	m := platforms.NewMatcher(targetPlatform)
	return m.Match(descPlatform)
}

// Matcher returns a ggcr matcher that selects images matching platform p (using
// containerd/platform matching rules) and non-image descriptors.
func Matcher(p *ggcrv1.Platform) match.Matcher {
	return func(desc ggcrv1.Descriptor) bool {
		if p != nil && desc.MediaType.IsImage() {
			return DescriptorSatisfies(desc, *p)
		}
		return true
	}
}
