// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package ordersfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
)

const (
	unsentFilePrefix  = "unsent-orders-"
	archiveFilePrefix = "archived-orders-"
)

var (
	// Error identifies errors with orders files.
	Error = errs.Class("ordersfile")

	mon = monkit.Package()
)

// Info contains full information about an order.
type Info struct {
	Limit *pb.OrderLimit
	Order *pb.Order
}

// Writable defines an interface for a write-only orders file.
type Writable interface {
	Append(*Info) error
	Close() error
}

// Readable defines an interface for a read-only orders file.
type Readable interface {
	ReadOne() (*Info, error)
	Close() error
}

// OpenWritableUnsent creates or opens for appending the unsent orders file for a given satellite ID and creation hour.
func OpenWritableUnsent(log *zap.Logger, unsentDir string, satelliteID storj.NodeID, creationTime time.Time) (Writable, error) {
	fileName := unsentFileName(satelliteID, creationTime)
	filePath := filepath.Join(unsentDir, fileName)

	// create file if not exists or append
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &fileV0{
		log: log.Named("writable V0 orders file"),
		f:   f,
	}, nil
}

// UnsentInfo contains information relevant to an unsent orders file, as well as information necessary to open it for reading.
type UnsentInfo struct {
	SatelliteID   storj.NodeID
	CreatedAtHour time.Time
}

// ArchivedInfo contains information relevant to an archived orders file, as well as information necessary to open it for reading.
type ArchivedInfo struct {
	SatelliteID   storj.NodeID
	CreatedAtHour time.Time
	ArchivedAt    time.Time
	StatusText    string
}

// GetUnsentInfo returns a new UnsentInfo which can be used to get information about and read from an unsent orders file.
func GetUnsentInfo(info os.FileInfo) (*UnsentInfo, error) {
	satelliteID, createdAtHour, err := getUnsentFileInfo(info.Name())
	if err != nil {
		return nil, err
	}

	return &UnsentInfo{
		SatelliteID:   satelliteID,
		CreatedAtHour: createdAtHour,
	}, nil
}

// GetArchivedInfo returns a new ArchivedInfo which can be used to get information about and read from an archived orders file.
func GetArchivedInfo(info os.FileInfo) (*ArchivedInfo, error) {
	satelliteID, createdAtHour, archivedAt, statusText, err := getArchivedFileInfo(info.Name())
	if err != nil {
		return nil, err
	}

	return &ArchivedInfo{
		SatelliteID:   satelliteID,
		CreatedAtHour: createdAtHour,
		ArchivedAt:    archivedAt,
		StatusText:    statusText,
	}, nil
}

// OpenReadable opens for reading the unsent or archived orders file at a given path.
// It assumes the path has already been validated with GetUnsentInfo or GetArchivedInfo.
func OpenReadable(log *zap.Logger, path string) (Readable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &fileV0{
		log: log.Named("readable V0 orders file"),
		f:   f,
	}, nil
}

// MoveUnsent moves an unsent orders file to the archived orders file directory.
func MoveUnsent(unsentDir, archiveDir string, satelliteID storj.NodeID, createdAtHour, archivedAt time.Time, status pb.SettlementWithWindowResponse_Status) error {
	oldFilePath := filepath.Join(unsentDir, unsentFileName(satelliteID, createdAtHour))
	newFilePath := filepath.Join(archiveDir, archiveFileName(satelliteID, createdAtHour, archivedAt, status))

	return Error.Wrap(os.Rename(oldFilePath, newFilePath))
}

// it expects the file name to be in the format "unsent-orders-<satelliteID>-<createdAtHour>".
func getUnsentFileInfo(filename string) (satellite storj.NodeID, createdHour time.Time, err error) {
	if !strings.HasPrefix(filename, unsentFilePrefix) {
		return storj.NodeID{}, time.Time{}, Error.New("invalid path: %q", filename)
	}
	// chop off prefix to get satellite ID and created hours
	infoStr := filename[len(unsentFilePrefix):]
	infoSlice := strings.Split(infoStr, "-")
	if len(infoSlice) != 2 {
		return storj.NodeID{}, time.Time{}, Error.New("invalid path: %q", filename)
	}

	satelliteIDStr := infoSlice[0]
	satelliteID, err := storj.NodeIDFromString(satelliteIDStr)
	if err != nil {
		return storj.NodeID{}, time.Time{}, Error.New("invalid path: %q", filename)
	}

	timeStr := infoSlice[1]
	createdHourUnixNano, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		return satelliteID, time.Time{}, Error.Wrap(err)
	}
	createdAtHour := time.Unix(0, createdHourUnixNano)

	return satelliteID, createdAtHour, nil
}

// getArchivedFileInfo gets the archived at time from an archive file name.
// it expects the file name to be in the format "archived-orders-<satelliteID>-<createdAtHour>-<archviedAtTime>-<status>".
func getArchivedFileInfo(name string) (satelliteID storj.NodeID, createdAtHour, archivedAt time.Time, status string, err error) {
	if !strings.HasPrefix(name, archiveFilePrefix) {
		return storj.NodeID{}, time.Time{}, time.Time{}, "", Error.New("invalid path: %q", name)
	}
	// chop off prefix to get satellite ID, created hour, archive time, and status
	infoStr := name[len(archiveFilePrefix):]
	infoSlice := strings.Split(infoStr, "-")
	if len(infoSlice) != 4 {
		return storj.NodeID{}, time.Time{}, time.Time{}, "", Error.New("invalid path: %q", name)
	}

	satelliteIDStr := infoSlice[0]
	satelliteID, err = storj.NodeIDFromString(satelliteIDStr)
	if err != nil {
		return storj.NodeID{}, time.Time{}, time.Time{}, "", Error.New("invalid path: %q", name)
	}

	createdAtStr := infoSlice[1]
	createdHourUnixNano, err := strconv.ParseInt(createdAtStr, 10, 64)
	if err != nil {
		return satelliteID, time.Time{}, time.Time{}, "", Error.New("invalid path: %q", name)
	}
	createdAtHour = time.Unix(0, createdHourUnixNano)

	archivedAtStr := infoSlice[2]
	archivedAtUnixNano, err := strconv.ParseInt(archivedAtStr, 10, 64)
	if err != nil {
		return satelliteID, createdAtHour, time.Time{}, "", Error.New("invalid path: %q", name)
	}
	archivedAt = time.Unix(0, archivedAtUnixNano)

	status = infoSlice[3]

	return satelliteID, createdAtHour, archivedAt, status, nil
}

func unsentFileName(satelliteID storj.NodeID, creationTime time.Time) string {
	return fmt.Sprintf("%s%s-%s",
		unsentFilePrefix,
		satelliteID,
		getCreationHourString(creationTime),
	)
}

func archiveFileName(satelliteID storj.NodeID, creationTime, archiveTime time.Time, status pb.SettlementWithWindowResponse_Status) string {
	return fmt.Sprintf("%s%s-%s-%s-%s",
		archiveFilePrefix,
		satelliteID,
		getCreationHourString(creationTime),
		strconv.FormatInt(archiveTime.UnixNano(), 10),
		pb.SettlementWithWindowResponse_Status_name[int32(status)],
	)
}

func getCreationHourString(t time.Time) string {
	creationHour := date.TruncateToHourInNano(t)
	timeStr := strconv.FormatInt(creationHour, 10)
	return timeStr
}
