// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"database/sql/driver"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/spannerutil"
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
	// ErrMethodNotAllowed general error when operation is not allowed.
	ErrMethodNotAllowed = errs.Class("method not allowed")
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
	BucketName BucketName
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
		BucketName: BucketName(elements[1]),
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
	loc.BucketName = BucketName(compactPrefix[len(loc.ProjectID):])
	return loc, nil
}

// Prefix converts bucket location into bucket prefix.
func (loc BucketLocation) Prefix() BucketPrefix {
	return BucketPrefix(loc.ProjectID.String() + "/" + loc.BucketName.String())
}

// CompactPrefix converts bucket location into bucket prefix with compact project ID.
func (loc BucketLocation) CompactPrefix() []byte {
	xs := make([]byte, 0, len(loc.ProjectID)+len(loc.BucketName))
	xs = append(xs, loc.ProjectID[:]...)
	xs = append(xs, []byte(loc.BucketName)...)
	return xs
}

// Compare compares this BucketLocation with another.
func (loc BucketLocation) Compare(other BucketLocation) int {
	cmp := loc.ProjectID.Compare(other.ProjectID)
	if cmp != 0 {
		return cmp
	}
	return loc.BucketName.Compare(other.BucketName)
}

// BucketName is a plain-text string, however we should treat it as unsafe bytes to
// avoid issues with any encoding.
type BucketName string

// String implements stringer func.
func (b BucketName) String() string { return string(b) }

// Compare implements comparison for bucket names.
func (b BucketName) Compare(x BucketName) int {
	return strings.Compare(b.String(), x.String())
}

// Value converts a BucketName to a database field.
func (b BucketName) Value() (driver.Value, error) {
	return []byte(b), nil
}

// Scan extracts a BucketName from a database field.
func (b *BucketName) Scan(value interface{}) error {
	switch value := value.(type) {
	case []byte:
		*b = BucketName(value)
		return nil
	default:
		return Error.New("unable to scan %T into BucketName", value)
	}
}

// EncodeSpanner implements spanner.Encoder.
func (b BucketName) EncodeSpanner() (any, error) {
	return string(b), nil
}

// DecodeSpanner implements spanner.Decoder.
func (b *BucketName) DecodeSpanner(value any) error {
	if x, ok := value.(string); ok {
		*b = BucketName(x)
		return nil
	}
	return Error.New("unable to scan %T into BucketName", value)
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

// EncodeSpanner implements spanner.Encoder.
func (o ObjectKey) EncodeSpanner() (any, error) {
	return o.Value()
}

// DecodeSpanner implements spanner.Decoder.
func (o *ObjectKey) DecodeSpanner(value any) error {
	if base64Val, ok := value.(string); ok {
		bytesVal, err := base64.StdEncoding.DecodeString(base64Val)
		if err != nil {
			return Error.Wrap(err)
		}
		*o = ObjectKey(bytesVal)
		return nil
	}
	return Error.New("unable to scan %T into ObjectKey", value)
}

// ObjectLocation is decoded object key information.
type ObjectLocation struct {
	ProjectID  uuid.UUID
	BucketName BucketName
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
	BucketName BucketName
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
		BucketName: BucketName(elements[2]),
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
		seg.BucketName.String(),
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
	BucketName BucketName
	ObjectKey  ObjectKey
	Version    Version
	StreamID   uuid.UUID
}

// Less implements sorting on object streams.
// Where ProjectID asc, BucketName asc, ObjectKey asc, Version desc.
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
		return obj.Version > b.Version
	}
	return obj.StreamID.Less(b.StreamID)
}

// LessVersionAsc implements sorting on object streams.
// Where ProjectID asc, BucketName asc, ObjectKey asc, Version asc.
func (obj ObjectStream) LessVersionAsc(b ObjectStream) bool {
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
	BucketName BucketName
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

// DecodeSpanner implements spanner.Decoder.
func (pos *SegmentPosition) DecodeSpanner(val any) (err error) {
	switch value := val.(type) {
	case int64:
		*pos = SegmentPositionFromEncoded(uint64(value))
	case string:
		parsedValue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return Error.New("unable to scan %T into SegmentPosition: %v", val, err)
		}
		*pos = SegmentPositionFromEncoded(uint64(parsedValue))
	default:
		return Error.New("unable to scan %T into SegmentPosition", val)
	}
	return nil
}

// EncodeSpanner implements spanner.Encoder.
func (pos SegmentPosition) EncodeSpanner() (any, error) {
	return int64(pos.Encode()), nil
}

// Version is used to uniquely identify objects with the same key.
type Version int64

// NextVersion means that the version should be chosen automatically.
const NextVersion = Version(0)

// DefaultVersion represents default version 1.
const DefaultVersion = Version(1)

// PendingVersion represents version that is used for pending objects (with UsePendingObjects).
const PendingVersion = Version(0)

// MaxVersion represents maximum version.
// Version in DB is represented as INT8.
//
// It uses `MaxInt64 - 64` to avoid issues with `-MaxVersion`.
const MaxVersion = Version(math.MaxInt64 - 64)

// Retention represents an object version's Object Lock retention configuration.
type Retention struct {
	Mode        storj.RetentionMode
	RetainUntil time.Time
}

// Enabled returns whether the retention configuration is enabled.
func (r *Retention) Enabled() bool {
	return r.Mode != storj.NoRetention
}

// Active returns whether the retention configuration is enabled and active as of the given time.
func (r *Retention) Active(now time.Time) bool {
	return r.Enabled() && now.Before(r.RetainUntil)
}

// ActiveNow returns whether the retention configuration is enabled and active as of the current time.
func (r *Retention) ActiveNow() bool {
	return r.Active(time.Now())
}

// Verify verifies the retention configuration.
func (r *Retention) Verify() error {
	if r.Mode == storj.GovernanceMode {
		if r.RetainUntil.IsZero() {
			return errs.New("retention period expiration must be set if retention mode is set")
		}
		return nil
	}
	return r.verifyWithoutGovernance()
}

// verifyWithoutGovernance verifies the retention configuration. It's used by metabase DB methods that haven't
// yet been adjusted to support governance mode, so it treats governance mode as invalid.
func (r *Retention) verifyWithoutGovernance() error {
	switch r.Mode {
	case storj.ComplianceMode:
		if r.RetainUntil.IsZero() {
			return errs.New("retention period expiration must be set if retention mode is set")
		}
	case storj.NoRetention:
		if !r.RetainUntil.IsZero() {
			return errs.New("retention period expiration must not be set if retention mode is not set")
		}
	default:
		return errs.New("invalid retention mode %d", r.Mode)
	}
	return nil
}

// StreamVersionID represents combined Version and StreamID suffix for purposes of public API.
// First 8 bytes represents Version and rest are object StreamID suffix.
// TODO(ver): we may consider renaming this type to VersionID but to do that
// we would need to rename metabase.Version into metabase.SequenceNumber/metabase.Sequence to
// avoid confusion.
type StreamVersionID uuid.UUID

// Version returns Version encoded into stream version id.
func (s StreamVersionID) Version() Version {
	return Version(binary.BigEndian.Uint64(s[:8]))
}

// StreamIDSuffix returns StreamID suffix encoded into stream version id.
func (s StreamVersionID) StreamIDSuffix() []byte {
	return s[8:]
}

// Bytes returnes stream version id bytes.
func (s StreamVersionID) Bytes() []byte {
	return s[:]
}

// NewStreamVersionID returns a new stream version id.
func NewStreamVersionID(version Version, streamID uuid.UUID) StreamVersionID {
	var sv StreamVersionID
	binary.BigEndian.PutUint64(sv[:8], uint64(version))
	copy(sv[8:], streamID[8:])
	return sv
}

// StreamVersionIDFromBytes decodes stream version id from bytes.
func StreamVersionIDFromBytes(bytes []byte) (_ StreamVersionID, err error) {
	if len(bytes) != len(StreamVersionID{}) {
		return StreamVersionID{}, ErrInvalidRequest.New("invalid stream version id")
	}
	var sv StreamVersionID
	copy(sv[:], bytes)
	return sv, nil
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
	// DeleteMarkerVersioned is inserted when an object is deleted in a versioning enabled bucket.
	DeleteMarkerVersioned = ObjectStatus(5)
	// DeleteMarkerUnversioned is inserted when an unversioned object is deleted in a versioning suspended bucket.
	DeleteMarkerUnversioned = ObjectStatus(6)

	// Prefix is an ephemeral status used during non-recursive listing.
	Prefix = ObjectStatus(7)

	// Constants that can be used while constructing SQL queries.
	statusPending                 = "1"
	statusCommittedUnversioned    = "3"
	statusCommittedVersioned      = "4"
	statusesCommitted             = "(" + statusCommittedUnversioned + "," + statusCommittedVersioned + ")"
	statusDeleteMarkerVersioned   = "5"
	statusDeleteMarkerUnversioned = "6"
	statusesDeleteMarker          = "(" + statusDeleteMarkerUnversioned + "," + statusDeleteMarkerVersioned + ")"
	statusesUnversioned           = "(" + statusCommittedUnversioned + "," + statusDeleteMarkerUnversioned + ")"

	retentionModeNone                        = "0"
	retentionModeCompliance                  = "1"
	retentionModeGovernance                  = "2"
	retentionModeComplianceAndGovernanceMask = "3"
	retentionModeLegalHold                   = "4"
	retentionModesComplianceAndGovernance    = "(" + retentionModeCompliance + "," + retentionModeGovernance + ")"
)

func committedWhereVersioned(versioned bool) ObjectStatus {
	if versioned {
		return CommittedVersioned
	}
	return CommittedUnversioned
}

// IsDeleteMarker return whether the status is a delete marker.
func (status ObjectStatus) IsDeleteMarker() bool {
	return status == DeleteMarkerUnversioned || status == DeleteMarkerVersioned
}

// IsUnversioned returns whether the status indicates that an object is unversioned.
func (status ObjectStatus) IsUnversioned() bool {
	return status == DeleteMarkerUnversioned || status == CommittedUnversioned
}

// IsCommitted returns whether the status indicates that an object is committed.
func (status ObjectStatus) IsCommitted() bool {
	return status == CommittedUnversioned || status == CommittedVersioned
}

// String returns textual representation of status.
func (status ObjectStatus) String() string {
	switch status {
	case Pending:
		return "Pending"
	case ObjectStatus(2):
		return "Deleted" // Deprecated
	case CommittedUnversioned:
		return "CommittedUnversioned"
	case CommittedVersioned:
		return "CommittedVersioned"
	case DeleteMarkerVersioned:
		return "DeleteMarkerVersioned"
	case DeleteMarkerUnversioned:
		return "DeleteMarkerUnversioned"
	case Prefix:
		return "Prefix"
	default:
		return fmt.Sprintf("ObjectStatus(%d)", int(status))
	}
}

// EncodeSpanner implements spanner.Encoder.
func (status ObjectStatus) EncodeSpanner() (any, error) {
	return int64(status), nil
}

// DecodeSpanner implements spanner.Decoder.
func (status *ObjectStatus) DecodeSpanner(val any) (err error) {
	return spannerutil.Int(status).DecodeSpanner(val)
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
