// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"database/sql/driver"
	"encoding/binary"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

var (
	// Error is the default error for metabase.
	Error = errs.Class("metabase")
	// ErrObjectAlreadyExists is used to indicate that object already exists.
	ErrObjectAlreadyExists = errs.Class("object already exists")
	// ErrPendingObjectMissing is used to indicate a pending object is no longer accessible.
	ErrPendingObjectMissing = errs.Class("pending object missing")
	// ErrPermissionDenied general error for denying permission.
	ErrPermissionDenied = errs.Class("permission denied")
)

// Common constants for segment keys.
const (
	Delimiter        = '/'
	LastSegmentName  = "l"
	LastSegmentIndex = uint32(math.MaxUint32)
)

// ListLimit is the maximum number of items the client can request for listing.
const ListLimit = intLimitRange(1000)

// MoveSegmentLimit is the maximum number of segments that can be moved.
const MoveSegmentLimit = int64(10000)

// CopySegmentLimit is the maximum number of segments that can be copied.
const CopySegmentLimit = int64(10000)

// batchsizeLimit specifies up to how many items fetch from the storage layer at
// a time.
//
// NOTE: A frequent pattern while listing items is to list up to ListLimit items
// and see whether there is more by trying to fetch another one. If the caller
// requests a list of ListLimit size and batchSizeLimit equals ListLimit, we
// would have queried another batch on that check for more items. Most of these
// results, except the first one, would be thrown away by callers. To prevent
// this from happening, we add 1 to batchSizeLimit.
const batchsizeLimit = ListLimit + 1

// BucketPrefix consists of <project id>/<bucket name>.
type BucketPrefix string

// BucketLocation defines a bucket that belongs to a project.
type BucketLocation struct {
	ProjectID  uuid.UUID
	BucketName string
}

// ParseBucketPrefix parses BucketPrefix.
func ParseBucketPrefix(prefix BucketPrefix) (BucketLocation, error) {
	elements := strings.Split(string(prefix), "/")
	if len(elements) != 2 {
		return BucketLocation{}, Error.New("invalid prefix %q", prefix)
	}

	projectID, err := uuid.FromString(elements[0])
	if err != nil {
		return BucketLocation{}, Error.Wrap(err)
	}

	return BucketLocation{
		ProjectID:  projectID,
		BucketName: elements[1],
	}, nil
}

// Verify object location fields.
func (loc BucketLocation) Verify() error {
	switch {
	case loc.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case loc.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	}
	return nil
}

// ParseCompactBucketPrefix parses BucketPrefix.
func ParseCompactBucketPrefix(compactPrefix []byte) (BucketLocation, error) {
	if len(compactPrefix) < len(uuid.UUID{}) {
		return BucketLocation{}, Error.New("invalid prefix %q", compactPrefix)
	}

	var loc BucketLocation
	copy(loc.ProjectID[:], compactPrefix)
	loc.BucketName = string(compactPrefix[len(loc.ProjectID):])
	return loc, nil
}

// Prefix converts bucket location into bucket prefix.
func (loc BucketLocation) Prefix() BucketPrefix {
	return BucketPrefix(loc.ProjectID.String() + "/" + loc.BucketName)
}

// CompactPrefix converts bucket location into bucket prefix with compact project ID.
func (loc BucketLocation) CompactPrefix() []byte {
	xs := make([]byte, 0, len(loc.ProjectID)+len(loc.BucketName))
	xs = append(xs, loc.ProjectID[:]...)
	xs = append(xs, []byte(loc.BucketName)...)
	return xs
}

// ObjectKey is an encrypted object key encoded using Path Component Encoding.
// It is not ascii safe.
type ObjectKey string

// Value converts a ObjectKey to a database field.
func (o ObjectKey) Value() (driver.Value, error) {
	return []byte(o), nil
}

// Scan extracts a ObjectKey from a database field.
func (o *ObjectKey) Scan(value interface{}) error {
	switch value := value.(type) {
	case []byte:
		*o = ObjectKey(value)
		return nil
	default:
		return Error.New("unable to scan %T into ObjectKey", value)
	}
}

// ObjectLocation is decoded object key information.
type ObjectLocation struct {
	ProjectID  uuid.UUID
	BucketName string
	ObjectKey  ObjectKey
}

// Bucket returns bucket location this object belongs to.
func (obj ObjectLocation) Bucket() BucketLocation {
	return BucketLocation{
		ProjectID:  obj.ProjectID,
		BucketName: obj.BucketName,
	}
}

// Verify object location fields.
func (obj ObjectLocation) Verify() error {
	switch {
	case obj.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case obj.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case len(obj.ObjectKey) == 0:
		return ErrInvalidRequest.New("ObjectKey missing")
	}
	return nil
}

// SegmentKey is an encoded metainfo key. This is used as the key in pointerdb key-value store.
type SegmentKey []byte

// SegmentLocation is decoded segment key information.
type SegmentLocation struct {
	ProjectID  uuid.UUID
	BucketName string
	ObjectKey  ObjectKey
	Position   SegmentPosition
}

// Bucket returns bucket location this segment belongs to.
func (seg SegmentLocation) Bucket() BucketLocation {
	return BucketLocation{
		ProjectID:  seg.ProjectID,
		BucketName: seg.BucketName,
	}
}

// Object returns the object location associated with this segment location.
func (seg SegmentLocation) Object() ObjectLocation {
	return ObjectLocation{
		ProjectID:  seg.ProjectID,
		BucketName: seg.BucketName,
		ObjectKey:  seg.ObjectKey,
	}
}

// ParseSegmentKey parses an segment key into segment location.
func ParseSegmentKey(encoded SegmentKey) (SegmentLocation, error) {
	elements := strings.SplitN(string(encoded), "/", 4)
	if len(elements) < 4 {
		return SegmentLocation{}, Error.New("invalid key %q", encoded)
	}

	projectID, err := uuid.FromString(elements[0])
	if err != nil {
		return SegmentLocation{}, Error.New("invalid key %q", encoded)
	}

	var position SegmentPosition
	if elements[1] == LastSegmentName {
		position.Index = LastSegmentIndex
	} else {
		if !strings.HasPrefix(elements[1], "s") {
			return SegmentLocation{}, Error.New("invalid %q, missing segment prefix in %q", string(encoded), elements[1])
		}
		// skip 's' prefix from segment index we got
		parsed, err := strconv.ParseUint(elements[1][1:], 10, 64)
		if err != nil {
			return SegmentLocation{}, Error.New("invalid %q, segment number %q", string(encoded), elements[1])
		}
		position = SegmentPositionFromEncoded(parsed)
	}

	return SegmentLocation{
		ProjectID:  projectID,
		BucketName: elements[2],
		Position:   position,
		ObjectKey:  ObjectKey(elements[3]),
	}, nil
}

// Encode converts segment location into a segment key.
func (seg SegmentLocation) Encode() SegmentKey {
	segment := LastSegmentName
	if seg.Position.Index != LastSegmentIndex {
		segment = "s" + strconv.FormatUint(seg.Position.Encode(), 10)
	}
	return SegmentKey(storj.JoinPaths(
		seg.ProjectID.String(),
		segment,
		seg.BucketName,
		string(seg.ObjectKey),
	))
}

// Verify segment location fields.
func (seg SegmentLocation) Verify() error {
	switch {
	case seg.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case seg.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case len(seg.ObjectKey) == 0:
		return ErrInvalidRequest.New("ObjectKey missing")
	}
	return nil
}

// ObjectStream uniquely defines an object and stream.
type ObjectStream struct {
	ProjectID  uuid.UUID
	BucketName string
	ObjectKey  ObjectKey
	Version    Version
	StreamID   uuid.UUID
}

// Less implements sorting on object streams.
func (obj ObjectStream) Less(b ObjectStream) bool {
	if obj.ProjectID != b.ProjectID {
		return obj.ProjectID.Less(b.ProjectID)
	}
	if obj.BucketName != b.BucketName {
		return obj.BucketName < b.BucketName
	}
	if obj.ObjectKey != b.ObjectKey {
		return obj.ObjectKey < b.ObjectKey
	}
	if obj.Version != b.Version {
		return obj.Version < b.Version
	}
	return obj.StreamID.Less(b.StreamID)
}

// Verify object stream fields.
func (obj *ObjectStream) Verify() error {
	switch {
	case obj.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case obj.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case len(obj.ObjectKey) == 0:
		return ErrInvalidRequest.New("ObjectKey missing")
	case obj.Version < 0:
		return ErrInvalidRequest.New("Version invalid: %v", obj.Version)
	case obj.StreamID.IsZero():
		return ErrInvalidRequest.New("StreamID missing")
	}
	return nil
}

// Location returns object location.
func (obj *ObjectStream) Location() ObjectLocation {
	return ObjectLocation{
		ProjectID:  obj.ProjectID,
		BucketName: obj.BucketName,
		ObjectKey:  obj.ObjectKey,
	}
}

// PendingObjectStream uniquely defines an pending object and stream.
type PendingObjectStream struct {
	ProjectID  uuid.UUID
	BucketName string
	ObjectKey  ObjectKey
	StreamID   uuid.UUID
}

// Verify object stream fields.
func (obj *PendingObjectStream) Verify() error {
	switch {
	case obj.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case obj.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case len(obj.ObjectKey) == 0:
		return ErrInvalidRequest.New("ObjectKey missing")
	case obj.StreamID.IsZero():
		return ErrInvalidRequest.New("StreamID missing")
	}
	return nil
}

// SegmentPosition is segment part and index combined.
type SegmentPosition struct {
	Part  uint32
	Index uint32
}

// SegmentPositionFromEncoded decodes an uint64 into a SegmentPosition.
func SegmentPositionFromEncoded(v uint64) SegmentPosition {
	return SegmentPosition{
		Part:  uint32(v >> 32),
		Index: uint32(v),
	}
}

// Encode encodes a segment position into an uint64, that can be stored in a database.
func (pos SegmentPosition) Encode() uint64 { return uint64(pos.Part)<<32 | uint64(pos.Index) }

// Less returns whether pos should before b.
func (pos SegmentPosition) Less(b SegmentPosition) bool { return pos.Encode() < b.Encode() }

// Version is used to uniquely identify objects with the same key.
type Version int64

// NextVersion means that the version should be chosen automatically.
const NextVersion = Version(0)

// DefaultVersion represents default version 1.
const DefaultVersion = Version(1)

// PendingVersion represents version that is used for pending objects (with UsePendingObjects).
const PendingVersion = Version(0)

// MaxVersion represents maximum version.
// Version in DB is represented as INT4.
const MaxVersion = Version(math.MaxInt32)

// Encode encodes version to bytes.
// TODO(ver): this is not final approach to version encoding. It's simplified
// version for internal testing purposes. Will be changed in future.
func (v Version) Encode() []byte {
	var bytes [8]byte
	binary.BigEndian.PutUint64(bytes[:], uint64(v))
	return bytes[:]
}

// VersionFromBytes decodes version from bytes.
func VersionFromBytes(bytes []byte) (Version, error) {
	if len(bytes) != 8 {
		return Version(0), ErrInvalidRequest.New("invalid version")
	}
	return Version(binary.BigEndian.Uint64(bytes)), nil
}

// ObjectStatus defines the status that the object is in.
//
// There are two types of objects:
//   - Regular (i.e. Committed), which is used for storing data.
//   - Delete Marker, which is used to show that an object has been deleted, while preserving older versions.
//
// Each object can be in two states:
//   - Pending, meaning that it's still being uploaded.
//   - Committed, meaning it has finished uploading.
//     Delete Markers are always considered committed, because they do not require committing.
//
// There are two options for versioning:
//   - Unversioned, there's only one allowed per project, bucket and encryption key.
//   - Versioned, there can be any number of such objects for a given project, bucket and encryption key.
//
// These lead to a few meaningful distinct statuses, listed below.
type ObjectStatus byte

const (
	// Pending means that the object is being uploaded or that the client failed during upload.
	// The failed upload may be continued in the future.
	Pending = ObjectStatus(1)
	// Committing used to one of the stages, which is not in use anymore.
	_ = ObjectStatus(2)
	// CommittedUnversioned means that the object is finished and should be visible for general listing.
	CommittedUnversioned = ObjectStatus(3)
	// CommittedVersioned means that the object is finished and should be visible for general listing.
	CommittedVersioned = ObjectStatus(4)
	// DeleteMarkerUnversioned is inserted when an unversioned object is deleted in a versioning suspended bucket.
	DeleteMarkerUnversioned = ObjectStatus(5)
	// DeleteMarkerVersioned is inserted when an object is deleted in a versioning enabled bucket.
	DeleteMarkerVersioned = ObjectStatus(6)
	// Prefix is an ephemeral status used during non-recursive listing.
	Prefix = ObjectStatus(7)

	// Constants that can be used while constructing SQL queries.
	statusPending                 = "1"
	statusCommittedUnversioned    = "3"
	statusCommittedVersioned      = "4"
	statusesCommitted             = "(3,4)"
	statusDeleteMarkerUnversioned = "5"
	statusDeleteMarkerVersioned   = "6"
	statusesDeleteMarker          = "(5,6)"
	statusesUnversioned           = "(3,5)"
)

func committedWhereVersioned(versioned bool) ObjectStatus {
	if versioned {
		return CommittedVersioned
	}
	return CommittedUnversioned
}

// stub uses so the linter wouldn't complain.
var (
	_ = CommittedVersioned
	_ = DeleteMarkerUnversioned
	_ = DeleteMarkerVersioned
	_ = statusCommittedVersioned
	_ = statusesCommitted
	_ = statusDeleteMarkerUnversioned
	_ = statusDeleteMarkerVersioned
	_ = statusesDeleteMarker
)

// IsDeleteMarker return whether the status is a delete marker.
func (status ObjectStatus) IsDeleteMarker() bool {
	return status == DeleteMarkerUnversioned || status == DeleteMarkerVersioned
}

// Pieces defines information for pieces.
type Pieces []Piece

// Piece defines information for a segment piece.
type Piece struct {
	Number      uint16
	StorageNode storj.NodeID
}

// Verify verifies pieces.
func (p Pieces) Verify() error {
	if len(p) == 0 {
		return ErrInvalidRequest.New("pieces missing")
	}

	currentPiece := p[0]
	if currentPiece.StorageNode == (storj.NodeID{}) {
		return ErrInvalidRequest.New("piece number %d is missing storage node id", currentPiece.Number)
	}

	for _, piece := range p[1:] {
		switch {
		case piece.Number == currentPiece.Number:
			return ErrInvalidRequest.New("duplicated piece number %d", piece.Number)
		case piece.Number < currentPiece.Number:
			return ErrInvalidRequest.New("pieces should be ordered")
		case piece.StorageNode == (storj.NodeID{}):
			return ErrInvalidRequest.New("piece number %d is missing storage node id", piece.Number)
		}
		currentPiece = piece
	}
	return nil
}

// Equal checks if Pieces structures are equal.
func (p Pieces) Equal(pieces Pieces) bool {
	if len(p) != len(pieces) {
		return false
	}

	first := make(Pieces, len(p))
	second := make(Pieces, len(p))

	copy(first, p)
	copy(second, pieces)

	sort.Slice(first, func(i, j int) bool {
		return first[i].Number < first[j].Number
	})
	sort.Slice(second, func(i, j int) bool {
		return second[i].Number < second[j].Number
	})

	for i := range first {
		if first[i].Number != second[i].Number {
			return false
		}
		if first[i].StorageNode != second[i].StorageNode {
			return false
		}
	}

	return true
}

// Len is the number of pieces.
func (p Pieces) Len() int { return len(p) }

// Less reports whether the piece with
// index i should sort before the piece with index j.
func (p Pieces) Less(i, j int) bool { return p[i].Number < p[j].Number }

// Swap swaps the pieces with indexes i and j.
func (p Pieces) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// Add adds the specified pieces and returns the updated Pieces.
func (p Pieces) Add(piecesToAdd Pieces) (Pieces, error) {
	return p.Update(piecesToAdd, nil)
}

// Remove removes the specified pieces from the original pieces
// and returns the updated Pieces.
func (p Pieces) Remove(piecesToRemove Pieces) (Pieces, error) {
	if len(p) == 0 {
		return Pieces{}, ErrInvalidRequest.New("pieces missing")
	}
	return p.Update(nil, piecesToRemove)
}

// Update adds piecesToAdd pieces and removes piecesToRemove pieces from
// the original pieces struct and returns the updated Pieces.
//
// It removes the piecesToRemove only if all piece number, node id match.
//
// When adding a piece, it checks if the piece already exists using the piece Number
// If a piece already exists, it returns an empty pieces struct and an error.
func (p Pieces) Update(piecesToAdd, piecesToRemove Pieces) (Pieces, error) {
	pieceMap := make(map[uint16]Piece)
	for _, piece := range p {
		pieceMap[piece.Number] = piece
	}

	// remove the piecesToRemove from the map
	// only if all piece number, node id match
	for _, piece := range piecesToRemove {
		if piece == (Piece{}) {
			continue
		}
		existing := pieceMap[piece.Number]
		if existing != (Piece{}) && existing.StorageNode == piece.StorageNode {
			delete(pieceMap, piece.Number)
		}
	}

	// add the piecesToAdd to the map
	for _, piece := range piecesToAdd {
		if piece == (Piece{}) {
			continue
		}
		_, exists := pieceMap[piece.Number]
		if exists {
			return Pieces{}, Error.New("piece to add already exists (piece no: %d)", piece.Number)
		}
		pieceMap[piece.Number] = piece
	}

	newPieces := make(Pieces, 0, len(pieceMap))
	for _, piece := range pieceMap {
		newPieces = append(newPieces, piece)
	}
	sort.Sort(newPieces)

	return newPieces, nil
}

// FindByNum finds a piece among the Pieces with the given piece number.
// If no such piece is found, `found` will be returned false.
func (p Pieces) FindByNum(pieceNum int) (_ Piece, found bool) {
	for _, piece := range p {
		if int(piece.Number) == pieceNum {
			return piece, true
		}
	}
	return Piece{}, false
}
