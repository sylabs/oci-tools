// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sourcesink

import (
	"testing"

	cosignoci "github.com/sigstore/cosign/v2/pkg/oci"
)

func Test_sifDescriptor_CosignImages(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		recursive bool
		wantCount int
	}{
		{
			name:      "Image",
			src:       corpus.SIF(t, "hello-world-cosign-manifest"),
			recursive: false,
			wantCount: 2, // 1 signature, 1 attestation against image
		},
		{
			name:      "IndexOnly",
			src:       corpus.SIF(t, "hello-world-cosign-manifest-list"),
			recursive: false,
			wantCount: 2, // 1 signature, 1 attestation against index
		},
		{
			name:      "IndexRecursive",
			src:       corpus.SIF(t, "hello-world-cosign-manifest-list"),
			recursive: true,
			wantCount: 11, // 1 signature, 1 attestation against index + 1 signature against each referenced image (9 total)
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := SIFFromPath(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			d, err := s.Get(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			sd, ok := d.(SignedDescriptor)
			if !ok {
				t.Fatal("could not upgrade Descriptor to SignedDescriptor")
			}

			got, err := sd.CosignImages(t.Context(), tt.recursive)
			if err != nil {
				t.Fatal(err)
			}

			if len(got) != tt.wantCount {
				t.Errorf("Got %d cosign images, expected %d", len(got), tt.wantCount)
			}
		})
	}
}

//nolint:dupl
func Test_sifDescriptor_SignedImage(t *testing.T) {
	tests := []struct {
		name             string
		src              string
		wantSignatures   int
		wantAttestations int
	}{
		{
			name:             "UnsignedImage",
			src:              corpus.SIF(t, "hello-world-docker-v2-manifest"),
			wantSignatures:   0,
			wantAttestations: 0,
		},
		{
			name:             "SignedImage",
			src:              corpus.SIF(t, "hello-world-cosign-manifest"),
			wantSignatures:   1,
			wantAttestations: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := SIFFromPath(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			d, err := s.Get(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			sd, ok := d.(SignedDescriptor)
			if !ok {
				t.Fatal("could not upgrade Descriptor to SignedDescriptor")
			}

			si, err := sd.SignedImage(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			checkSignedImage(t, si, tt.wantSignatures, tt.wantAttestations)
		})
	}
}

func checkSignedImage(t *testing.T, si cosignoci.SignedImage, wantSigs, wantAtts int) {
	t.Helper()
	sig, err := si.Signatures()
	if err != nil {
		t.Fatal(err)
	}
	sigs, err := sig.Get()
	if err != nil {
		t.Fatal(err)
	}
	if len(sigs) != wantSigs {
		t.Errorf("Got %d cosign signatures, expected %d", len(sigs), wantSigs)
	}

	att, err := si.Attestations()
	if err != nil {
		t.Fatal(err)
	}
	atts, err := att.Get()
	if err != nil {
		t.Fatal(err)
	}
	if len(atts) != wantAtts {
		t.Errorf("Got %d cosign attestations, expected %d", len(atts), wantAtts)
	}
}

//nolint:dupl
func Test_sifDescriptor_SignedImageIndex(t *testing.T) {
	tests := []struct {
		name             string
		src              string
		wantSignatures   int
		wantAttestations int
	}{
		{
			name:             "UnsignedIndex",
			src:              corpus.SIF(t, "hello-world-docker-v2-manifest-list"),
			wantSignatures:   0,
			wantAttestations: 0,
		},
		{
			name:             "SignedIndex",
			src:              corpus.SIF(t, "hello-world-cosign-manifest-list"),
			wantSignatures:   1,
			wantAttestations: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := SIFFromPath(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			d, err := s.Get(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			sd, ok := d.(SignedDescriptor)
			if !ok {
				t.Fatal("could not upgrade Descriptor to SignedDescriptor")
			}

			sii, err := sd.SignedImageIndex(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			checkSignedImageIndex(t, sii, tt.wantSignatures, tt.wantAttestations)
		})
	}
}

func checkSignedImageIndex(t *testing.T, sii cosignoci.SignedImageIndex, wantSigs, wantAtts int) {
	t.Helper()
	sig, err := sii.Signatures()
	if err != nil {
		t.Fatal(err)
	}
	sigs, err := sig.Get()
	if err != nil {
		t.Fatal(err)
	}
	if len(sigs) != wantSigs {
		t.Errorf("Got %d cosign signatures, expected %d", len(sigs), wantSigs)
	}

	att, err := sii.Attestations()
	if err != nil {
		t.Fatal(err)
	}
	atts, err := att.Get()
	if err != nil {
		t.Fatal(err)
	}
	if len(atts) != wantAtts {
		t.Errorf("Got %d cosign attestations, expected %d", len(atts), wantAtts)
	}
}
