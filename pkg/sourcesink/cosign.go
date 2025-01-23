// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sourcesink

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	cosignoci "github.com/sigstore/cosign/v2/pkg/oci"
	cosignremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
)

// SignedDescriptor provides access to cosign signatures stored against it.
type SignedDescriptor interface {
	// CosignImages checks for image manifests providing cosign signatures &
	// attestations that are associated with the image or index with the descriptor
	// If image manifests providing cosign signatures and / or attestations
	// exist, then these images are returned in a name.Reference -> v1.Image map.
	//
	// If recursive is true, then if the descriptor is an index, we also check for
	// signatures and attestations for each of its associated manifests.
	//
	// In the returned map, the images are referenced as '_cosign:<tag>', where
	// <tag> matches the tag at src. The '_cosign' repository placeholder string
	// is used instead of any original registry & repository names.
	CosignImages(ctx context.Context, recursive bool) ([]ReferencedImage, error)
	// SignedImage wraps an image Descriptor as a cosign oci.SignedImage,
	// allowing access to signatures and attestations stored alongside the image.
	SignedImage(context.Context) (cosignoci.SignedImage, error)
	// SignedImageIndex wraps an image index Descriptor as a cosign oci.SignedImageIndex,
	// allowing access to signatures and attestations stored alongside the image.
	SignedImageIndex(context.Context) (cosignoci.SignedImageIndex, error)
}

// CosignPlaceholderRepo is a placeholder repository name for cosign images.
const CosignPlaceholderRepo = "_cosign"

type ReferencedImage struct {
	Ref name.Reference
	Img v1.Image
}

func NumDescriptorsForCosign(imgs []ReferencedImage) (int64, error) {
	descCount := int64(0)
	for _, ri := range imgs {
		ls, err := ri.Img.Layers()
		if err != nil {
			return 0, err
		}
		descCount += int64(len(ls) + 2)
	}
	return descCount, nil
}

//nolint:gochecknoglobals
var cosignSuffixes = []string{
	cosignremote.SignatureTagSuffix,
	cosignremote.AttestationTagSuffix,
}

func CosignTag(h v1.Hash, suffix string) string {
	return fmt.Sprint(h.Algorithm, "-", h.Hex, ".", suffix)
}

func CosignRef(imgDigest v1.Hash, imgRef name.Reference, suffix string, opts ...name.Option) (name.Reference, error) {
	t := CosignTag(imgDigest, suffix)
	repo := CosignPlaceholderRepo
	if imgRef != nil {
		repo = imgRef.Context().Name()
	}
	opts = append(opts, name.WithDefaultRegistry(""))
	return name.ParseReference(repo+":"+t, opts...)
}
