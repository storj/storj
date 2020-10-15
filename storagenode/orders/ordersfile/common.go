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

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
)

// Version is a type for defining different file versions.
type Version string

const (
	// V0 is the first orders file version. It stores orders and limits with no checksum.
	V0 = Version("v0")
	// V1 is the second orders file version. It includes a checksum for each entry so that file corruption is handled better.
	V1 = Version("v1")

	unsentFilePrefix  = "unsent-orders-"
	archiveFilePrefix = "archived-orders-"
)

var (
	// Error identifies errors with orders files.
	Error = errs.Class("ordersfile")
	// ErrEntryCorrupt is returned when a corrupt entry is found.
	ErrEntryCorrupt = errs.Class("ordersfile corrupt entry")
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
func OpenWritableUnsent(unsentDir string, satelliteID storj.NodeID, creationTime time.Time) (Writable, error) {
	// if V0 file already exists, use that. Otherwise use V1 file.
	versionToUse := V0
	fileName := UnsentFileName(satelliteID, creationTime, V0)
	filePath := filepath.Join(unsentDir, fileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fileName = UnsentFileName(satelliteID, creationTime, V1)
		filePath = filepath.Join(unsentDir, fileName)
		versionToUse = V1
	}

	if versionToUse == V0 {
		return OpenWritableV0(filePath)
	}
	return OpenWritableV1(filePath, satelliteID, creationTime)
}

// UnsentInfo contains information relevant to an unsent orders file, as well as information necessary to open it for reading.
type UnsentInfo struct {
	SatelliteID   storj.NodeID
	CreatedAtHour time.Time
	Version       Version
}

// ArchivedInfo contains information relevant to an archived orders file, as well as information necessary to open it for reading.
type ArchivedInfo struct {
	SatelliteID   storj.NodeID
	CreatedAtHour time.Time
	ArchivedAt    time.Time
	StatusText    string
	Version       Version
}

// GetUnsentInfo returns a new UnsentInfo which can be used to get information about and read from an unsent orders file.
func GetUnsentInfo(info os.FileInfo) (*UnsentInfo, error) {
	satelliteID, createdAtHour, version, err := getUnsentFileInfo(info.Name())
	if err != nil {
		return nil, err
	}

	return &UnsentInfo{
		SatelliteID:   satelliteID,
		CreatedAtHour: createdAtHour,
		Version:       version,
	}, nil
}

// GetArchivedInfo returns a new ArchivedInfo which can be used to get information about and read from an archived orders file.
func GetArchivedInfo(info os.FileInfo) (*ArchivedInfo, error) {
	satelliteID, createdAtHour, archivedAt, statusText, version, err := getArchivedFileInfo(info.Name())
	if err != nil {
		return nil, err
	}

	return &ArchivedInfo{
		SatelliteID:   satelliteID,
		CreatedAtHour: createdAtHour,
		ArchivedAt:    archivedAt,
		StatusText:    statusText,
		Version:       version,
	}, nil
}

// OpenReadable opens for reading the unsent or archived orders file at a given path.
// It assumes the path has already been validated with GetUnsentInfo or GetArchivedInfo.
func OpenReadable(path string, version Version) (Readable, error) {
	if version == V0 {
		return OpenReadableV0(path)
	}
	return OpenReadableV1(path)
}

// MoveUnsent moves an unsent orders file to the archived orders file directory.
func MoveUnsent(unsentDir, archiveDir string, satelliteID storj.NodeID, createdAtHour, archivedAt time.Time, status pb.SettlementWithWindowResponse_Status, version Version) error {
	oldFilePath := filepath.Join(unsentDir, UnsentFileName(satelliteID, createdAtHour, version))
	newFilePath := filepath.Join(archiveDir, ArchiveFileName(satelliteID, createdAtHour, archivedAt, status, version))

	return Error.Wrap(os.Rename(oldFilePath, newFilePath))
}

// it expects the file name to be in the format "unsent-orders-<satelliteID>-<createdAtHour>.<version>".
// V0 will not have ".<version>" at the end of the filename.
func getUnsentFileInfo(filename string) (satellite storj.NodeID, createdHour time.Time, version Version, err error) {
	filename, version = getVersion(filename)

	if !strings.HasPrefix(filename, unsentFilePrefix) {
		return storj.NodeID{}, time.Time{}, version, Error.New("invalid path: %q", filename)
	}
	// chop off prefix to get satellite ID and created hours
	infoStr := filename[len(unsentFilePrefix):]
	infoSlice := strings.Split(infoStr, "-")
	if len(infoSlice) != 2 {
		return storj.NodeID{}, time.Time{}, version, Error.New("invalid path: %q", filename)
	}

	satelliteIDStr := infoSlice[0]
	satelliteID, err := storj.NodeIDFromString(satelliteIDStr)
	if err != nil {
		return storj.NodeID{}, time.Time{}, version, Error.New("invalid path: %q", filename)
	}

	timeStr := infoSlice[1]
	createdHourUnixNano, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		return satelliteID, time.Time{}, version, Error.Wrap(err)
	}
	createdAtHour := time.Unix(0, createdHourUnixNano)

	return satelliteID, createdAtHour, version, nil
}

// getArchivedFileInfo gets the archived at time from an archive file name.
// it expects the file name to be in the format "archived-orders-<satelliteID>-<createdAtHour>-<archviedAtTime>-<status>.<version>".
// V0 will not have ".<version>" at the end of the filename.
func getArchivedFileInfo(name string) (satelliteID storj.NodeID, createdAtHour, archivedAt time.Time, status string, version Version, err error) {
	name, version = getVersion(name)

	if !strings.HasPrefix(name, archiveFilePrefix) {
		return storj.NodeID{}, time.Time{}, time.Time{}, "", version, Error.New("invalid path: %q", name)
	}
	// chop off prefix to get satellite ID, created hour, archive time, and status
	infoStr := name[len(archiveFilePrefix):]
	infoSlice := strings.Split(infoStr, "-")
	if len(infoSlice) != 4 {
		return storj.NodeID{}, time.Time{}, time.Time{}, "", version, Error.New("invalid path: %q", name)
	}

	satelliteIDStr := infoSlice[0]
	satelliteID, err = storj.NodeIDFromString(satelliteIDStr)
	if err != nil {
		return storj.NodeID{}, time.Time{}, time.Time{}, "", version, Error.New("invalid path: %q", name)
	}

	createdAtStr := infoSlice[1]
	createdHourUnixNano, err := strconv.ParseInt(createdAtStr, 10, 64)
	if err != nil {
		return satelliteID, time.Time{}, time.Time{}, "", version, Error.New("invalid path: %q", name)
	}
	createdAtHour = time.Unix(0, createdHourUnixNano)

	archivedAtStr := infoSlice[2]
	archivedAtUnixNano, err := strconv.ParseInt(archivedAtStr, 10, 64)
	if err != nil {
		return satelliteID, createdAtHour, time.Time{}, "", version, Error.New("invalid path: %q", name)
	}
	archivedAt = time.Unix(0, archivedAtUnixNano)

	status = infoSlice[3]

	return satelliteID, createdAtHour, archivedAt, status, version, nil
}

// UnsentFileName gets the filename of an unsent file.
func UnsentFileName(satelliteID storj.NodeID, creationTime time.Time, version Version) string {
	filename := fmt.Sprintf("%s%s-%s",
		unsentFilePrefix,
		satelliteID,
		getCreationHourString(creationTime),
	)
	if version != V0 {
		filename = fmt.Sprintf("%s.%s", filename, version)
	}
	return filename
}

// ArchiveFileName gets the filename of an archived file.
func ArchiveFileName(satelliteID storj.NodeID, creationTime, archiveTime time.Time, status pb.SettlementWithWindowResponse_Status, version Version) string {
	filename := fmt.Sprintf("%s%s-%s-%s-%s",
		archiveFilePrefix,
		satelliteID,
		getCreationHourString(creationTime),
		strconv.FormatInt(archiveTime.UnixNano(), 10),
		pb.SettlementWithWindowResponse_Status_name[int32(status)],
	)
	if version != V0 {
		filename = fmt.Sprintf("%s.%s", filename, version)
	}
	return filename
}

func getCreationHourString(t time.Time) string {
	creationHour := date.TruncateToHourInNano(t)
	timeStr := strconv.FormatInt(creationHour, 10)
	return timeStr
}

func getVersion(filename string) (trimmed string, version Version) {
	ext := filepath.Ext(filename)
	if ext == "."+string(V1) {
		version = V1
		trimmed = strings.TrimSuffix(filename, ext)
		return trimmed, V1
	}
	return filename, V0
}
