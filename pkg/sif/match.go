// Copyright 2024 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"errors"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

var (
	ErrNoMatch         = errors.New("no match found")
	ErrMultipleMatches = errors.New("multiple matches found")
)

func matchAll(v1.Descriptor) bool { return true }
