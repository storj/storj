// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"crypto/x509"
	"crypto/x509/pkix"

	"storj.io/common/storj"
)

// Bucket contains information about a specific bucket
type Bucket = storj.Bucket

// EncryptionParameters is the cipher suite and parameters used for encryption
type EncryptionParameters = storj.EncryptionParameters

// CipherSuite specifies one of the encryption suites supported by Storj
// libraries for encryption of in-network data.
type CipherSuite = storj.CipherSuite

const (
	// EncUnspecified indicates no encryption suite has been selected.
	EncUnspecified = storj.EncUnspecified
	// EncNull indicates use of the NULL cipher; that is, no encryption is
	// done. The ciphertext is equal to the plaintext.
	EncNull = storj.EncNull
	// EncAESGCM indicates use of AES128-GCM encryption.
	EncAESGCM = storj.EncAESGCM
	// EncSecretBox indicates use of XSalsa20-Poly1305 encryption, as provided
	// by the NaCl cryptography library under the name "Secretbox".
	EncSecretBox = storj.EncSecretBox
	// EncNullBase64URL is like EncNull but Base64 encodes/decodes the
	// binary path data (URL-safe)
	EncNullBase64URL = storj.EncNullBase64URL
)

// Constant definitions for key and nonce sizes
const (
	KeySize   = storj.KeySize
	NonceSize = storj.NonceSize
)

// NewKey creates a new Storj key from humanReadableKey.
func NewKey(humanReadableKey []byte) (*Key, error) {
	return storj.NewKey(humanReadableKey)
}

// Key represents the largest key used by any encryption protocol
type Key = storj.Key

// Nonce represents the largest nonce used by any encryption protocol
type Nonce = storj.Nonce

// NonceFromString decodes an base32 encoded
func NonceFromString(s string) (Nonce, error) {
	return storj.NonceFromString(s)
}

// NonceFromBytes converts a byte slice into a nonce
func NonceFromBytes(b []byte) (Nonce, error) {
	return storj.NonceFromBytes(b)
}

// EncryptedPrivateKey is a private key that has been encrypted
type EncryptedPrivateKey = storj.EncryptedPrivateKey

// V0 represents identity version 0
// NB: identities created before identity versioning (i.e. which don't have a
// version extension; "legacy") will be recognized as V0.
const V0 = storj.V0

// IDVersionNumber is the number of an identity version.
type IDVersionNumber = storj.IDVersionNumber

// IDVersion holds fields that are used to distinguish different identity
// versions from one another; used in identity generation.
type IDVersion = storj.IDVersion

// GetIDVersion looks up the given version number in the map of registered
// versions, returning an error if none is found.
func GetIDVersion(number IDVersionNumber) (IDVersion, error) {
	return storj.GetIDVersion(number)
}

// LatestIDVersion returns the last IDVersion registered.
func LatestIDVersion() IDVersion {
	return storj.LatestIDVersion()
}

// IDVersionFromCert parsed the IDVersion from the passed certificate's IDVersion extension.
func IDVersionFromCert(cert *x509.Certificate) (IDVersion, error) {
	return storj.IDVersionFromCert(cert)
}

// IDVersionInVersions returns an error if the given version is in the given string of version(s)/range(s).
func IDVersionInVersions(versionNumber IDVersionNumber, versionsStr string) error {
	return storj.IDVersionInVersions(versionNumber, versionsStr)
}

// ListDirection specifies listing direction
type ListDirection = storj.ListDirection

const (
	// Before lists backwards from cursor, without cursor [NOT SUPPORTED]
	Before = storj.Before
	// Backward lists backwards from cursor, including cursor [NOT SUPPORTED]
	Backward = storj.Backward
	// Forward lists forwards from cursor, including cursor
	Forward = storj.Forward
	// After lists forwards from cursor, without cursor
	After = storj.After
)

// ListOptions lists objects
type ListOptions = storj.ListOptions

// ObjectList is a list of objects
type ObjectList = storj.ObjectList

// BucketListOptions lists objects
type BucketListOptions = storj.BucketListOptions

// BucketList is a list of buckets
type BucketList = storj.BucketList

// NodeIDSize is the byte length of a NodeID
const NodeIDSize = storj.NodeIDSize

// NodeID is a unique node identifier
type NodeID = storj.NodeID

// NodeIDList is a slice of NodeIDs (implements sort)
type NodeIDList = storj.NodeIDList

// NewVersionedID adds an identity version to a node ID.
func NewVersionedID(id NodeID, version IDVersion) NodeID {
	return storj.NewVersionedID(id, version)
}

// NewVersionExt creates a new identity version certificate extension for the
// given identity version,
func NewVersionExt(version IDVersion) pkix.Extension {
	return storj.NewVersionExt(version)
}

// NodeIDFromString decodes a base58check encoded node id string
func NodeIDFromString(s string) (NodeID, error) {
	return storj.NodeIDFromString(s)
}

// NodeIDsFromBytes converts a 2d byte slice into a list of nodes
func NodeIDsFromBytes(b [][]byte) (ids NodeIDList, err error) {
	return storj.NodeIDsFromBytes(b)
}

// NodeIDFromBytes converts a byte slice into a node id
func NodeIDFromBytes(b []byte) (NodeID, error) {
	return storj.NodeIDFromBytes(b)
}

// NodeURL defines a structure for connecting to a node.
type NodeURL = storj.NodeURL

// ParseNodeURL parses node URL string.
//
// Examples:
//
//    raw IP:
//      33.20.0.1:7777
//      [2001:db8:1f70::999:de8:7648:6e8]:7777
//
//    with NodeID:
//      12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@33.20.0.1:7777
//      12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@[2001:db8:1f70::999:de8:7648:6e8]:7777
//
//    without host:
//      12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@
func ParseNodeURL(s string) (NodeURL, error) {
	return storj.ParseNodeURL(s)
}

// NodeURLs defines a comma delimited flag for defining a list node url-s.
type NodeURLs = storj.NodeURLs

// ParseNodeURLs parses comma delimited list of node urls
func ParseNodeURLs(s string) (NodeURLs, error) {
	return storj.ParseNodeURLs(s)
}

// Object contains information about a specific object
type Object = storj.Object

// ObjectInfo contains information about a specific object
type ObjectInfo = storj.ObjectInfo

// Stream is information about an object stream
type Stream = storj.Stream

// LastSegment contains info about last segment
// TODO: remove
type LastSegment = storj.LastSegment

// Segment is full segment information
type Segment = storj.Segment

// Piece is information where a piece is located
type Piece = storj.Piece

// ObjectListItem represents listed object
type ObjectListItem = storj.ObjectListItem

// Path represents a object path
type Path = storj.Path

// SplitPath splits path into a slice of path components
func SplitPath(path Path) []string {
	return storj.SplitPath(path)
}

// JoinPaths concatenates paths to a new single path
func JoinPaths(paths ...Path) Path {
	return storj.JoinPaths(paths...)
}

// PieceID is the unique identifier for pieces
type PieceID = storj.PieceID

// NewPieceID creates a piece ID
func NewPieceID() PieceID {
	return storj.NewPieceID()
}

// PieceIDFromString decodes a hex encoded piece ID string
func PieceIDFromString(s string) (PieceID, error) {
	return storj.PieceIDFromString(s)
}

// PieceIDFromBytes converts a byte slice into a piece ID
func PieceIDFromBytes(b []byte) (PieceID, error) {
	return storj.PieceIDFromBytes(b)
}

// PiecePublicKey is the unique identifier for pieces
type PiecePublicKey = storj.PiecePublicKey

// PiecePrivateKey is the unique identifier for pieces
type PiecePrivateKey = storj.PiecePrivateKey

// NewPieceKey creates a piece key pair
func NewPieceKey() (PiecePublicKey, PiecePrivateKey, error) {
	return storj.NewPieceKey()
}

// PiecePublicKeyFromBytes converts bytes to a piece public key.
func PiecePublicKeyFromBytes(data []byte) (PiecePublicKey, error) {
	return storj.PiecePublicKeyFromBytes(data)
}

// PiecePrivateKeyFromBytes converts bytes to a piece private key.
func PiecePrivateKeyFromBytes(data []byte) (PiecePrivateKey, error) {
	return storj.PiecePrivateKeyFromBytes(data)
}

// RedundancyScheme specifies the parameters and the algorithm for redundancy
type RedundancyScheme = storj.RedundancyScheme

// RedundancyAlgorithm is the algorithm used for redundancy
type RedundancyAlgorithm = storj.RedundancyAlgorithm

// List of supported redundancy algorithms
const (
	InvalidRedundancyAlgorithm = storj.InvalidRedundancyAlgorithm
	ReedSolomon                = storj.ReedSolomon
)

// SegmentPosition segment position in object
type SegmentPosition = storj.SegmentPosition

// SegmentListItem represents listed segment
type SegmentListItem = storj.SegmentListItem

// SegmentDownloadInfo represents segment download information inline/remote
type SegmentDownloadInfo = storj.SegmentDownloadInfo

// SegmentEncryption represents segment encryption key and nonce
type SegmentEncryption = storj.SegmentEncryption

// SegmentID is the unique identifier for segment related to object
type SegmentID = storj.SegmentID

// SegmentIDFromString decodes an base32 encoded
func SegmentIDFromString(s string) (SegmentID, error) {
	return storj.SegmentIDFromString(s)
}

// SegmentIDFromBytes converts a byte slice into a segment ID
func SegmentIDFromBytes(b []byte) (SegmentID, error) {
	return storj.SegmentIDFromBytes(b)
}

// SerialNumber is the unique identifier for pieces
type SerialNumber = storj.SerialNumber

// SerialNumberFromString decodes an base32 encoded
func SerialNumberFromString(s string) (SerialNumber, error) {
	return storj.SerialNumberFromString(s)
}

// SerialNumberFromBytes converts a byte slice into a serial number
func SerialNumberFromBytes(b []byte) (SerialNumber, error) {
	return storj.SerialNumberFromBytes(b)
}

// StreamID is the unique identifier for stream related to object
type StreamID = storj.StreamID

// StreamIDFromString decodes an base32 encoded
func StreamIDFromString(s string) (StreamID, error) {
	return storj.StreamIDFromString(s)
}

// StreamIDFromBytes converts a byte slice into a stream ID
func StreamIDFromBytes(b []byte) (StreamID, error) {
	return storj.StreamIDFromBytes(b)
}
