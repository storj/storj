// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/gob"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
)

const (
	// Bucket is the bucket used with a bolt-backed authorizations DB.
	Bucket = "authorizations"
	// MaxClockSkew is the max duration in the past or future that a claim
	// timestamp is allowed to have and still be valid.
	MaxClockSkew    = 5 * time.Minute
	tokenDataLength = 64 // 2^(64*8) =~ 1.34E+154
	tokenDelimiter  = ":"
	tokenVersion    = 0
)

var (
	mon = monkit.Package()
	// Error is used when an error occurs involving an authorization.
	Error = errs.Class("authorization error")
	// ErrInvalidToken is used when a token is invalid.
	ErrInvalidToken = errs.Class("authorization token error")
)

// Group is a slice of authorizations for convenient de/serialization.
// and grouping.
type Group []*Authorization

// Authorization represents a single-use authorization token and its status.
type Authorization struct {
	Token Token
	Claim *Claim
}

// Token is a userID and a random byte array, when serialized, can be used like
// a pre-shared key for claiming certificate signatures.
type Token struct {
	// NB: currently email address for convenience
	UserID string
	Data   [tokenDataLength]byte
}

// ClaimOpts hold parameters for claiming an authorization.
type ClaimOpts struct {
	Req           *pb.SigningRequest
	Peer          *rpcpeer.Peer
	ChainBytes    [][]byte
	MinDifficulty uint16
}

// Claim holds information about the circumstances under which an authorization
// token was claimed.
type Claim struct {
	Addr             string
	Timestamp        int64
	Identity         *identity.PeerIdentity
	SignedChainBytes [][]byte
}

func init() {
	gob.Register(&ecdsa.PublicKey{})
	gob.Register(&rsa.PublicKey{})
	gob.Register(elliptic.P256())
}

// NewAuthorization creates a new, unclaimed authorization with a random token value.
func NewAuthorization(userID string) (*Authorization, error) {
	token := Token{UserID: userID}
	_, err := rand.Read(token.Data[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &Authorization{
		Token: token,
	}, nil
}

// ParseToken splits the token string on the delimiter to get a userID and data
// for a token and base58 decodes the data.
func ParseToken(tokenString string) (*Token, error) {
	splitAt := strings.LastIndex(tokenString, tokenDelimiter)
	if splitAt == -1 {
		return nil, ErrInvalidToken.New("delimiter missing")
	}

	userID, b58Data := tokenString[:splitAt], tokenString[splitAt+1:]
	if len(userID) == 0 {
		return nil, ErrInvalidToken.New("user ID missing")
	}

	data, _, err := base58.CheckDecode(b58Data)
	if err != nil {
		return nil, ErrInvalidToken.Wrap(err)
	}

	if len(data) != tokenDataLength {
		return nil, ErrInvalidToken.New("data size mismatch")
	}
	t := &Token{
		UserID: userID,
	}
	copy(t.Data[:], data)
	return t, nil
}

// Unmarshal deserializes a set of authorizations.
func (group *Group) Unmarshal(data []byte) error {
	decoder := gob.NewDecoder(bytes.NewBuffer(data))
	if err := decoder.Decode(group); err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// Marshal serializes a set of authorizations.
func (group Group) Marshal() ([]byte, error) {
	data := new(bytes.Buffer)
	encoder := gob.NewEncoder(data)
	err := encoder.Encode(group)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return data.Bytes(), nil
}

// GroupByClaimed separates a group of authorizations into a group of claimed
// and a group of open authorizations.
func (group Group) GroupByClaimed() (claimed, open Group) {
	for _, auth := range group {
		if auth.Claim != nil {
			// TODO: check if claim is valid? what if not?
			claimed = append(claimed, auth)
		} else {
			open = append(open, auth)
		}
	}
	return claimed, open
}

// String implements the stringer interface and prevents authorization data
// from completely leaking into logs and errors.
func (a Authorization) String() string {
	fmtLen := strconv.Itoa(len(a.Token.UserID) + 7)
	return fmt.Sprintf("%."+fmtLen+"s..", a.Token.String())
}

// Equal checks if two tokens have equal user IDs and data
func (t *Token) Equal(cmpToken *Token) bool {
	return t.UserID == cmpToken.UserID && bytes.Equal(t.Data[:], cmpToken.Data[:])
}

// String implements the stringer interface. Base68 w/ version and checksum bytes
// are used for easy and reliable human transport.
func (t *Token) String() string {
	return fmt.Sprintf("%s:%s", t.UserID, base58.CheckEncode(t.Data[:], tokenVersion))
}
