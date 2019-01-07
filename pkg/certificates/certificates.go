// Copyright (C) 2018 Storj Labs, Inc.
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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
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

// CertSigningConfig is a config struct for use with a certificate signing service client
type CertSigningConfig struct {
	AuthToken string `help:"authorization token to use to claim a certificate signing request (only applicable for the alpha network)"`
	Address   string `help:"address of the certificate signing rpc service"`
}

// CertSignerConfig is a config struct for use with a certificate signing service server
type CertSignerConfig struct {
	Overwrite          bool   `default:"false" help:"if true, overwrites config AND authorization db is truncated"`
	AuthorizationDBURL string `default:"bolt://$CONFDIR/authorizations.db" help:"url to the certificate signing authorization database"`
	MinDifficulty      uint   `default:"16" help:"minimum difficulty of the requester's identity required to claim an authorization"`
	CA                 identity.FullCAConfig
}

// CertificateSigner implements pb.CertificatesServer
type CertificateSigner struct {
	Log           *zap.Logger
	Signer        *identity.FullCertificateAuthority
	AuthDB        *AuthorizationDB
	MinDifficulty uint16
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
	client pb.CertificatesClient
}

func init() {
	gob.Register(&ecdsa.PublicKey{})
	gob.Register(elliptic.P256())
}

// NewClient creates a new certificate signing grpc client
func NewClient(ctx context.Context, ident *identity.FullIdentity, address string) (*Client, error) {
	tc := transport.NewClient(ident)
	conn, err := tc.DialAddress(ctx, address)
	if err != nil {
		return nil, err
	}

	return &Client{
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

// SetupIdentity loads or creates a CA and identity and submits a certificate
// signing request request for the CA; if successful, updated chains are saved.
func (c CertSigningConfig) SetupIdentity(
	ctx context.Context,
	caConfig identity.CASetupConfig,
	identConfig identity.SetupConfig,
) error {
	caStatus := caConfig.Status()
	var (
		ca    *identity.FullCertificateAuthority
		ident *identity.FullIdentity
		err   error
	)
	if caStatus == identity.CertKey && !caConfig.Overwrite {
		ca, err = caConfig.FullConfig().Load()
		if err != nil {
			return err
		}
	} else if caStatus != identity.NoCertNoKey && !caConfig.Overwrite {
		return identity.ErrSetup.New("certificate authority file(s) exist: %s", caStatus)
	} else {
		t, err := time.ParseDuration(caConfig.Timeout)
		if err != nil {
			return errs.Wrap(err)
		}
		ctx, cancel := context.WithTimeout(ctx, t)
		defer cancel()

		ca, err = caConfig.Create(ctx)
		if err != nil {
			return err
		}
	}

	identStatus := identConfig.Status()
	if identStatus == identity.CertKey && !identConfig.Overwrite {
		ident, err = identConfig.FullConfig().Load()
		if err != nil {
			return err
		}
	} else if identStatus != identity.NoCertNoKey && !identConfig.Overwrite {
		return identity.ErrSetup.New("identity file(s) exist: %s", identStatus)
	} else {
		ident, err = identConfig.Create(ca)
		if err != nil {
			return err
		}
	}

	signedChainBytes, err := c.Sign(ctx, ident)
	if err != nil {
		return errs.New("error occured while signing certificate: %s\n(identity files were still generated and saved, if you try again existnig files will be loaded)", err)
	}

	signedChain, err := identity.ParseCertChain(signedChainBytes)
	if err != nil {
		return nil
	}

	ca.Cert = signedChain[0]
	ca.RestChain = signedChain[1:]
	err = identity.FullCAConfig{
		CertPath: caConfig.FullConfig().CertPath,
	}.Save(ca)
	if err != nil {
		return err
	}

	ident.RestChain = signedChain[1:]
	err = identity.Config{
		CertPath: identConfig.FullConfig().CertPath,
	}.Save(ident)
	if err != nil {
		return err
	}
	return nil
}

// Sign submits a certificate signing request given the config
func (c CertSigningConfig) Sign(ctx context.Context, ident *identity.FullIdentity) ([][]byte, error) {
	client, err := NewClient(ctx, ident, c.Address)
	if err != nil {
		return nil, err
	}

	return client.Sign(ctx, c.AuthToken)
}

// Sign claims an authorization using the token string and returns a signed
// copy of the client's CA certificate
func (c Client) Sign(ctx context.Context, tokenStr string) ([][]byte, error) {
	res, err := c.client.Sign(ctx, &pb.SigningRequest{
		AuthToken: tokenStr,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		return nil, err
	}

	return res.Chain, nil
}

// NewAuthDB creates or opens the authorization database specified by the config
func (c CertSignerConfig) NewAuthDB() (*AuthorizationDB, error) {
	// TODO: refactor db selection logic?
	driver, source, err := utils.SplitDBURL(c.AuthorizationDBURL)
	if err != nil {
		return nil, peertls.ErrRevocationDB.Wrap(err)
	}

	authDB := new(AuthorizationDB)
	switch driver {
	case "bolt":
		_, err := os.Stat(source)
		if c.Overwrite && err == nil {
			if err := os.Remove(source); err != nil {
				return nil, err
			}
		}

		authDB.DB, err = boltdb.New(source, AuthorizationsBucket)
		if err != nil {
			return nil, ErrAuthorizationDB.Wrap(err)
		}
	case "redis":
		redisClient, err := redis.NewClientFrom(c.AuthorizationDBURL)
		if err != nil {
			return nil, ErrAuthorizationDB.Wrap(err)
		}

		if c.Overwrite {
			if err := redisClient.FlushDB(); err != nil {
				return nil, err
			}
		}

		authDB.DB = redisClient
	default:
		return nil, ErrAuthorizationDB.New("database scheme not supported: %s", driver)
	}

	return authDB, nil
}

// Run implements the responsibility interface, starting a certificate signing server.
func (c CertSignerConfig) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	authDB, err := c.NewAuthDB()
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, authDB.Close())
	}()

	signer, err := c.CA.Load()
	if err != nil {
		return err
	}

	srv := &CertificateSigner{
		Log:           zap.L(),
		Signer:        signer,
		AuthDB:        authDB,
		MinDifficulty: uint16(c.MinDifficulty),
	}
	pb.RegisterCertificatesServer(server.GRPC(), srv)

	srv.Log.Info(
		"Certificate signing server running",
		zap.String("address", server.Addr().String()),
	)

	go func() {
		done := ctx.Done()
		<-done
		if err := server.Close(); err != nil {
			srv.Log.Error("closing server", zap.Error(err))
		}
	}()

	return server.Run(ctx)
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

	signedPeerCA, err := c.Signer.Sign(peerIdent.CA)
	if err != nil {
		return nil, err
	}

	signedChainBytes := append(
		[][]byte{
			signedPeerCA.Raw,
			c.Signer.Cert.Raw,
		},
		c.Signer.RestChainRaw()...,
	)
	err = c.AuthDB.Claim(&ClaimOpts{
		Req:           req,
		Peer:          grpcPeer,
		ChainBytes:    signedChainBytes,
		MinDifficulty: c.MinDifficulty,
	})
	if err != nil {
		return nil, err
	}

	return &pb.SigningResponse{
		Chain: signedChainBytes,
	}, nil
}

// Close closes the authorization database's underlying store.
func (a *AuthorizationDB) Close() error {
	return ErrAuthorizationDB.Wrap(a.DB.Close())
}

// Create creates a new authorization and adds it to the authorization database.
func (a *AuthorizationDB) Create(userID string, count int) (Authorizations, error) {
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

	if err := a.add(userID, newAuths); err != nil {
		return nil, err
	}

	return newAuths, nil
}

// Get retrieves authorizations by user ID.
func (a *AuthorizationDB) Get(userID string) (Authorizations, error) {
	authsBytes, err := a.DB.Get(storage.Key(userID))
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
func (a *AuthorizationDB) UserIDs() ([]string, error) {
	keys, err := a.DB.List([]byte{}, 0)
	if err != nil {
		return nil, ErrAuthorizationDB.Wrap(err)
	}
	return keys.Strings(), nil
}

// Claim marks an authorization as claimed and records claim information
func (a *AuthorizationDB) Claim(opts *ClaimOpts) error {
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

	auths, err := a.Get(token.UserID)
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
			if err := a.put(token.UserID, auths); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (a *AuthorizationDB) add(userID string, newAuths Authorizations) error {
	auths, err := a.Get(userID)
	if err != nil {
		return err
	}

	auths = append(auths, newAuths...)
	return a.put(userID, auths)
}

func (a *AuthorizationDB) put(userID string, auths Authorizations) error {
	authsBytes, err := auths.Marshal()
	if err != nil {
		return ErrAuthorizationDB.Wrap(err)
	}

	if err := a.DB.Put(storage.Key(userID), authsBytes); err != nil {
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
