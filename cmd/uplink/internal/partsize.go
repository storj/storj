// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package internal

import (
	"github.com/zeebo/errs"

	"storj.io/common/memory"
)

const maxPartCount int64 = 10000

// PartSizeConfig represents the configuration for uploading a file in parts.
type PartSizeConfig struct {
	PartSize    int64
	SinglePart  bool
	Parallelism int
}

// CalculatePartSize returns the needed part size in order to upload the file with size of 'length'.
// It hereby respects if the client requests/prefers a certain size and only increases if needed.
func CalculatePartSize(contentLength, preferredPartSize int64, parallelism int) (cfg PartSizeConfig, err error) {
	const minimumPartSize = memory.GiB
	const alignPartSize = 64 * memory.MiB

	// Let the user pick their size if we don't have a contentLength to know better.
	if contentLength < 0 {
		partSize := preferredPartSize

		if partSize <= 0 { // user didn't pick a size
			partSize = minimumPartSize.Int64()
		}

		partSize = roundUpToNext(partSize, alignPartSize.Int64())

		return PartSizeConfig{
			PartSize:    partSize,
			SinglePart:  false,
			Parallelism: parallelism,
		}, nil
	}

	// When we are not parallel, there's no point in doing multipart upload.
	if parallelism <= 1 {
		return PartSizeConfig{
			PartSize:    contentLength,
			SinglePart:  true,
			Parallelism: 1,
		}, nil
	}

	// ceil(contentLength / maxPartCount)
	smallestAllowedPartSize := (contentLength + (maxPartCount - 1)) / maxPartCount

	// Calculate a good part size.
	partSize := roundUpToNext(smallestAllowedPartSize, alignPartSize.Int64())

	// Let's set a lower limit for the expected part size.
	if partSize < minimumPartSize.Int64() {
		partSize = minimumPartSize.Int64()
	}

	// check whether we can use preferred part size instead?
	if preferredPartSize > 0 {
		if preferredPartSize < partSize {
			return cfg, errs.New("the specified chunk size %s is too small, requires %s or larger",
				memory.FormatBytes(preferredPartSize), memory.FormatBytes(partSize))
		}

		partSize = roundUpToNext(preferredPartSize, alignPartSize.Int64())
	}

	cfg = PartSizeConfig{
		PartSize:    partSize,
		SinglePart:  contentLength <= partSize,
		Parallelism: parallelism,
	}

	// if there's a single part there's no point in allowing parallelism.
	if cfg.SinglePart {
		cfg.Parallelism = 1
	}

	return cfg, nil
}

func roundUpToNext(v, r int64) int64 {
	return ((v + (r - 1)) / r) * r
}
