// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package ordersfile

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

func TestOpenWritableUnsent(t *testing.T) {
	type args struct {
		unsentDir    string
		satelliteID  storj.NodeID
		creationTime time.Time
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1      Writable
		wantErr    bool
		inspectErr func(err error, t *testing.T)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1, err := OpenWritableUnsent(tArgs.unsentDir, tArgs.satelliteID, tArgs.creationTime)

			assert.Equal(t, tt.want1, got1)

			if tt.wantErr {
				require.Error(t, err)
				if tt.inspectErr != nil {
					tt.inspectErr(err, t)
				}
			}
		})
	}
}

func TestGetUnsentInfo(t *testing.T) {
	type args struct {
		info os.FileInfo
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1      *UnsentInfo
		wantErr    bool
		inspectErr func(err error, t *testing.T)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1, err := GetUnsentInfo(tArgs.info.Name())

			assert.Equal(t, tt.want1, got1)

			if tt.wantErr {
				require.Error(t, err)
				if tt.inspectErr != nil {
					tt.inspectErr(err, t)
				}
			}
		})
	}
}

func TestGetArchivedInfo(t *testing.T) {
	type args struct {
		info os.FileInfo
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1      *ArchivedInfo
		wantErr    bool
		inspectErr func(err error, t *testing.T)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1, err := GetArchivedInfo(tArgs.info.Name())

			assert.Equal(t, tt.want1, got1)

			if tt.wantErr {
				require.Error(t, err)
				if tt.inspectErr != nil {
					tt.inspectErr(err, t)
				}
			}
		})
	}
}

func TestOpenReadable(t *testing.T) {
	type args struct {
		path    string
		version Version
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1      Readable
		wantErr    bool
		inspectErr func(err error, t *testing.T)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1, err := OpenReadable(tArgs.path, tArgs.version)

			assert.Equal(t, tt.want1, got1)

			if tt.wantErr {
				require.Error(t, err)
				if tt.inspectErr != nil {
					tt.inspectErr(err, t)
				}
			}
		})
	}
}

func TestMoveUnsent(t *testing.T) {
	type args struct {
		unsentDir     string
		archiveDir    string
		satelliteID   storj.NodeID
		createdAtHour time.Time
		archivedAt    time.Time
		status        pb.SettlementWithWindowResponse_Status
		version       Version
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		wantErr    bool
		inspectErr func(err error, t *testing.T)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			err := MoveUnsent(tArgs.unsentDir, tArgs.archiveDir, tArgs.satelliteID, tArgs.createdAtHour, tArgs.archivedAt, tArgs.status, tArgs.version)

			if tt.wantErr {
				require.Error(t, err)
				if tt.inspectErr != nil {
					tt.inspectErr(err, t)
				}
			}
		})
	}
}

func TestGetUnsentFileInfo(t *testing.T) {
	type args struct {
		filename string
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1      storj.NodeID
		want2      time.Time
		want3      Version
		wantErr    bool
		inspectErr func(err error, t *testing.T)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1, got2, got3, err := getUnsentFileInfo(tArgs.filename)

			assert.Equal(t, tt.want1, got1)

			assert.Equal(t, tt.want2, got2)

			assert.Equal(t, tt.want3, got3)

			if tt.wantErr {
				require.Error(t, err)
				if tt.inspectErr != nil {
					tt.inspectErr(err, t)
				}
			}
		})
	}
}

func TestGetArchivedFileInfo(t *testing.T) {
	type args struct {
		name string
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1      storj.NodeID
		want2      time.Time
		want3      time.Time
		want4      string
		want5      Version
		wantErr    bool
		inspectErr func(err error, t *testing.T)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1, got2, got3, got4, got5, err := getArchivedFileInfo(tArgs.name)

			assert.Equal(t, tt.want1, got1)

			assert.Equal(t, tt.want2, got2)

			assert.Equal(t, tt.want3, got3)

			assert.Equal(t, tt.want4, got4)

			assert.Equal(t, tt.want5, got5)

			if tt.wantErr {
				require.Error(t, err)
				if tt.inspectErr != nil {
					tt.inspectErr(err, t)
				}
			}
		})
	}
}

func TestUnsentFileName(t *testing.T) {
	type args struct {
		satelliteID  storj.NodeID
		creationTime time.Time
		version      Version
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1 string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := UnsentFileName(tArgs.satelliteID, tArgs.creationTime, tArgs.version)

			assert.Equal(t, tt.want1, got1)
		})
	}
}

func TestArchiveFileName(t *testing.T) {
	type args struct {
		satelliteID  storj.NodeID
		creationTime time.Time
		archiveTime  time.Time
		status       pb.SettlementWithWindowResponse_Status
		version      Version
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1 string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := ArchiveFileName(tArgs.satelliteID, tArgs.creationTime, tArgs.archiveTime, tArgs.status, tArgs.version)

			assert.Equal(t, tt.want1, got1)
		})
	}
}

func TestGetCreationHourString(t *testing.T) {
	type args struct {
		t time.Time
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1 string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := getCreationHourString(tArgs.t)

			assert.Equal(t, tt.want1, got1)
		})
	}
}
