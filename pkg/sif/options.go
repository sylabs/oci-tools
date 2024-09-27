// Copyright 2024 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

// Option is a functional option for OCIFileImage operations.
type Option func(*options) error

type options struct{}
