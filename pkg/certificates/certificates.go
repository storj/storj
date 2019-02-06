// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificates

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

const (
	// AuthorizationsBucket is the bucket used with a bolt-backed authorizations DB.
	AuthorizationsBucket = "authorizations"
	// MaxClaimDelaySeconds is the max duration in seconds in the past or
	// future that a claim timestamp is allowed to have and still be valid.
	MaxClaimDelaySeconds = 15
	tokenDataLength      = 64 // 2^(64*8) =~ 1.34E+154
	tokenDelimiter       = ":"
	tokenVersion         = 0
)

var (
	mon = monkit.Package()
	// ErrAuthorization is used when an error occurs involving an authorization.
	ErrAuthorization = errs.Class("authorization error")
	// ErrAuthorizationDB is used when an error occurs involving the authorization database.
	ErrAuthorizationDB = errs.Class("authorization db error")
	// ErrInvalidToken is used when a token is invalid
	ErrInvalidToken = errs.Class("invalid token error")
	// ErrAuthorizationCount is used when attempting to create an invalid number of authorizations.
	ErrAuthorizationCount = ErrAuthorizationDB.New("cannot add less than one authorizations")
)

// CertificateSigner implements pb.CertificatesServer
type CertificateSigner struct {
	log           *zap.Logger
	signer        *identity.FullCertificateAuthority
	authDB        *AuthorizationDB
	minDifficulty uint16
}

// AuthorizationDB stores authorizations which may be claimed in exchange for a
// certificate signature.
type AuthorizationDB struct {
	DB storage.KeyValueStore
}

// Authorizations is a slice of authorizations for convenient de/serialization
// and grouping.
type Authorizations []*Authorization

// Authorization represents a single-use authorization token and its status
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

// ClaimOpts hold parameters for claiming an authorization
type ClaimOpts struct {
	Req           *pb.SigningRequest
	Peer          *peer.Peer
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

// Client implements pb.CertificateClient
type Client struct {
	conn   *grpc.ClientConn
	client pb.CertificatesClient
}

func init() {
	gob.Register(&ecdsa.PublicKey{})
	gob.Register(elliptic.P256())
}

// NewServer creates a new certificate signing grpc server
func NewServer(log *zap.Logger, signer *identity.FullCertificateAuthority, authDB *AuthorizationDB, minDifficulty uint16) *CertificateSigner {
	return &CertificateSigner{
		log:           log,
		signer:        signer,
		authDB:        authDB,
		minDifficulty: minDifficulty,
	}
}

// NewClient creates a new certificate signing grpc client
func NewClient(ctx context.Context, ident *identity.FullIdentity, address string) (*Client, error) {
	tc := transport.NewClient(ident)
	conn, err := tc.DialAddress(ctx, address)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:   conn,
		client: pb.NewCertificatesClient(conn),
	}, nil
}

// NewClientFrom creates a new certificate signing grpc client from an existing
// grpc cert signing client
func NewClientFrom(client pb.CertificatesClient) (*Client, error) {
	return &Client{
		client: client,
	}, nil
}

// NewAuthorization creates a new, unclaimed authorization with a random token value
func NewAuthorization(userID string) (*Authorization, error) {
	token := Token{UserID: userID}
	_, err := rand.Read(token.Data[:])
	if err != nil {
		return nil, ErrAuthorization.Wrap(err)
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

// Close closes the client
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Sign claims an authorization using the token string and returns a signed
// copy of the client's CA certificate
func (c *Client) Sign(ctx context.Context, tokenStr string) ([][]byte, error) {
	res, err := c.client.Sign(ctx, &pb.SigningRequest{
		AuthToken: tokenStr,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		return nil, err
	}

	return res.Chain, nil
}

// Sign signs a valid certificate signing request's cert.
func (c CertificateSigner) Sign(ctx context.Context, req *pb.SigningRequest) (*pb.SigningResponse, error) {
	grpcPeer, ok := peer.FromContext(ctx)
	if !ok {
		// TODO: better error
		return nil, errs.New("unable to get peer from context")
	}

	peerIdent, err := identity.PeerIdentityFromPeer(grpcPeer)
	if err != nil {
		return nil, err
	}

	signedPeerCA, err := c.signer.Sign(peerIdent.CA)
	if err != nil {
		return nil, err
	}

	signedChainBytes := [][]byte{signedPeerCA.Raw, c.signer.Cert.Raw}
	signedChainBytes = append(signedChainBytes, c.signer.RestChainRaw()...)
	err = c.authDB.Claim(&ClaimOpts{
		Req:           req,
		Peer:          grpcPeer,
		ChainBytes:    signedChainBytes,
		MinDifficulty: c.minDifficulty,
	})
	if err != nil {
		return nil, err
	}

	return &pb.SigningResponse{
		Chain: signedChainBytes,
	}, nil
}

// Close closes the authorization database's underlying store.
func (authDB *AuthorizationDB) Close() error {
	return ErrAuthorizationDB.Wrap(authDB.DB.Close())
}

// Create creates a new authorization and adds it to the authorization database.
func (authDB *AuthorizationDB) Create(userID string, count int) (Authorizations, error) {
	if len(userID) == 0 {
		return nil, ErrAuthorizationDB.New("userID cannot be empty")
	}
	if count < 1 {
		return nil, ErrAuthorizationCount
	}

	var (
		newAuths Authorizations
		authErrs utils.ErrorGroup
	)
	for i := 0; i < count; i++ {
		auth, err := NewAuthorization(userID)
		if err != nil {
			authErrs.Add(err)
			continue
		}
		newAuths = append(newAuths, auth)
	}
	if err := authErrs.Finish(); err != nil {
		return nil, ErrAuthorizationDB.Wrap(err)
	}

	if err := authDB.add(userID, newAuths); err != nil {
		return nil, err
	}

	return newAuths, nil
}

// Get retrieves authorizations by user ID.
func (authDB *AuthorizationDB) Get(userID string) (Authorizations, error) {
	authsBytes, err := authDB.DB.Get(storage.Key(userID))
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return nil, ErrAuthorizationDB.Wrap(err)
	}
	if authsBytes == nil {
		return nil, nil
	}

	var auths Authorizations
	if err := auths.Unmarshal(authsBytes); err != nil {
		return nil, ErrAuthorizationDB.Wrap(err)
	}
	return auths, nil
}

// UserIDs returns a list of all userIDs present in the authorization database.
func (authDB *AuthorizationDB) UserIDs() ([]string, error) {
	keys, err := authDB.DB.List([]byte{}, 0)
	if err != nil {
		return nil, ErrAuthorizationDB.Wrap(err)
	}
	return keys.Strings(), nil
}

// List returns all authorizations in the database.
func (authDB *AuthorizationDB) List() (auths Authorizations, _ error) {
	uids, err := authDB.UserIDs()
	if err != nil {
		return nil, err
	}

	for _, uid := range uids {
		idAuths, err := authDB.Get(uid)
		if err != nil {
			return nil, err
		}
		auths = append(auths, idAuths...)
	}
	return auths, nil
}

// Claim marks an authorization as claimed and records claim information.
func (authDB *AuthorizationDB) Claim(opts *ClaimOpts) error {
	now := time.Now().Unix()
	if !(now-MaxClaimDelaySeconds < opts.Req.Timestamp) ||
		!(opts.Req.Timestamp < now+MaxClaimDelaySeconds) {
		return ErrAuthorization.New("claim timestamp is outside of max delay window: %d", opts.Req.Timestamp)
	}

	ident, err := identity.PeerIdentityFromPeer(opts.Peer)
	if err != nil {
		return err
	}

	peerDifficulty, err := ident.ID.Difficulty()
	if err != nil {
		return err
	}

	if peerDifficulty < opts.MinDifficulty {
		return ErrAuthorization.New("difficulty must be greater than: %d", opts.MinDifficulty)
	}

	token, err := ParseToken(opts.Req.AuthToken)
	if err != nil {
		return err
	}

	auths, err := authDB.Get(token.UserID)
	if err != nil {
		return err
	}

	for i, auth := range auths {
		if auth.Token.Equal(token) {
			if auth.Claim != nil {
				return ErrAuthorization.New("authorization has already been claimed: %s", auth.String())
			}

			auths[i] = &Authorization{
				Token: auth.Token,
				Claim: &Claim{
					Timestamp:        now,
					Addr:             opts.Peer.Addr.String(),
					Identity:         ident,
					SignedChainBytes: opts.ChainBytes,
				},
			}
			if err := authDB.put(token.UserID, auths); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (authDB *AuthorizationDB) add(userID string, newAuths Authorizations) error {
	auths, err := authDB.Get(userID)
	if err != nil {
		return err
	}

	auths = append(auths, newAuths...)
	return authDB.put(userID, auths)
}

func (authDB *AuthorizationDB) put(userID string, auths Authorizations) error {
	authsBytes, err := auths.Marshal()
	if err != nil {
		return ErrAuthorizationDB.Wrap(err)
	}

	if err := authDB.DB.Put(storage.Key(userID), authsBytes); err != nil {
		return ErrAuthorizationDB.Wrap(err)
	}
	return nil
}

// Unmarshal deserializes a set of authorizations
func (a *Authorizations) Unmarshal(data []byte) error {
	decoder := gob.NewDecoder(bytes.NewBuffer(data))
	if err := decoder.Decode(a); err != nil {
		return ErrAuthorization.Wrap(err)
	}
	return nil
}

// Marshal serializes a set of authorizations
func (a Authorizations) Marshal() ([]byte, error) {
	data := new(bytes.Buffer)
	encoder := gob.NewEncoder(data)
	err := encoder.Encode(a)
	if err != nil {
		return nil, ErrAuthorization.Wrap(err)
	}

	return data.Bytes(), nil
}

// Group separates a set of authorizations into a set of claimed and a set of open authorizations.
func (a Authorizations) Group() (claimed, open Authorizations) {
	for _, auth := range a {
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
