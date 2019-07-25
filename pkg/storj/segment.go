// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

// SegmentPosition segment position in object
type SegmentPosition struct {
	PartNumber int32
	Index      int32
}

// SegmentListItem represents listed segment
type SegmentListItem struct {
	Position SegmentPosition
}

// SegmentDownloadInfo represents segment download information inline/remote
type SegmentDownloadInfo struct {
	SegmentID           SegmentID
	EncryptedInlineData []byte
	Next                SegmentPosition
}
