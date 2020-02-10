// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"time"

	"storj.io/storj/pkg/storj"
)

type NodeID storj.NodeID

func (nodeID NodeID) Bytes() []byte {
	return storj.NodeID(nodeID).Bytes()
}

func (nodeID NodeID) String() string {
	return storj.NodeID(nodeID).String()
}

func (nodeID *NodeID) UnmarshalCSV(s string) error {
	v, err := storj.NodeIDFromString(s)
	if err != nil {
		return err
	}
	*nodeID = NodeID(v)
	return nil
}

func (nodeID NodeID) MarshalCSV() (string, error) {
	return nodeID.String(), nil
}

type UTCDate time.Time

func (date UTCDate) String() string {
	return time.Time(date).Format("2006-01-02")
}

func (date *UTCDate) UnmarshalCSV(s string) error {
	v, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*date = UTCDate(v)
	return nil
}

func (date UTCDate) MarshalCSV() (string, error) {
	return date.String(), nil
}
