// Copyright 2024 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"errors"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
)

var (
	ErrNoMatch         = errors.New("no match found")
	ErrMultipleMatches = errors.New("multiple matches found")
)

func matchAll(v1.Descriptor) bool { return true }

// matchAllIfNil returns m if not nil, or a Matcher that matches all descriptors otherwise.
func matchAllIfNil(m match.Matcher) match.Matcher {
	if m != nil {
		return m
	}
	return matchAll
}
