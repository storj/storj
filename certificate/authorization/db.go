// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"context"
	"os"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/dbutil"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
)

// DB stores authorizations which may be claimed in exchange for a
// certificate signature.
type DB struct {
	db storage.KeyValueStore
}

// DBConfig is the authorization db config.
type DBConfig struct {
	URL       string `default:"bolt://$CONFDIR/authorizations.db" help:"url to the certificate signing authorization database"`
	Overwrite bool   `default:"false" help:"if true, overwrites config AND authorization db is truncated" setup:"true"`
}

// NewDBFromCfg creates and/or opens the authorization database specified by the config.
func NewDBFromCfg(config DBConfig) (*DB, error) {
	return NewDB(config.URL, config.Overwrite)
}

// NewDB creates and/or opens the authorization database.
func NewDB(dbURL string, overwrite bool) (*DB, error) {
	driver, source, err := dbutil.SplitConnstr(dbURL)
	if err != nil {
		return nil, extensions.ErrRevocationDB.Wrap(err)
	}

	authDB := new(DB)
	switch driver {
	case "bolt":
		_, err := os.Stat(source)
		if overwrite && err == nil {
			if err := os.Remove(source); err != nil {
				return nil, err
			}
		}

		authDB.db, err = boltdb.New(source, Bucket)
		if err != nil {
			return nil, ErrDB.Wrap(err)
		}
	case "redis":
		redisClient, err := redis.NewClientFrom(dbURL)
		if err != nil {
			return nil, ErrDB.Wrap(err)
		}

		if overwrite {
			if err := redisClient.FlushDB(); err != nil {
				return nil, err
			}
		}

		authDB.db = redisClient
	default:
		return nil, ErrDB.New("database scheme not supported: %s", driver)
	}

	return authDB, nil
}

// Close closes the authorization database's underlying store.
func (authDB *DB) Close() error {
	return ErrDB.Wrap(authDB.db.Close())
}

// Create creates a new authorization and adds it to the authorization database.
func (authDB *DB) Create(ctx context.Context, userID string, count int) (_ Group, err error) {
	defer mon.Task()(&ctx, userID, count)(&err)
	if len(userID) == 0 {
		return nil, ErrDB.Wrap(ErrEmptyUserID)
	}
	if count < 1 {
		return nil, ErrDB.Wrap(ErrCount)
	}

	var (
		newAuths Group
		authErrs errs.Group
	)
	for i := 0; i < count; i++ {
		auth, err := NewAuthorization(userID)
		if err != nil {
			authErrs.Add(err)
			continue
		}
		newAuths = append(newAuths, auth)
	}
	if authErrs.Err() != nil {
		return nil, ErrDB.Wrap(authErrs.Err())
	}

	if err := authDB.add(ctx, userID, newAuths); err != nil {
		return nil, err
	}

	return newAuths, nil
}

// Get retrieves authorizations by user ID.
func (authDB *DB) Get(ctx context.Context, userID string) (_ Group, err error) {
	defer mon.Task()(&ctx, userID)(&err)
	authsBytes, err := authDB.db.Get(ctx, storage.Key(userID))
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return nil, ErrDB.Wrap(err)
	}
	if authsBytes == nil {
		return nil, nil
	}

	var auths Group
	if err := auths.Unmarshal(authsBytes); err != nil {
		return nil, ErrDB.Wrap(err)
	}
	return auths, nil
}

// UserIDs returns a list of all userIDs present in the authorization database.
func (authDB *DB) UserIDs(ctx context.Context) (userIDs []string, err error) {
	defer mon.Task()(&ctx)(&err)
	err = authDB.db.Iterate(ctx, storage.IterateOptions{
		Recurse: true,
	}, func(ctx context.Context, iterator storage.Iterator) error {
		var listItem storage.ListItem
		for iterator.Next(ctx, &listItem) {
			userIDs = append(userIDs, listItem.Key.String())
		}
		return nil
	})
	return userIDs, err
}

// List returns all authorizations in the database.
func (authDB *DB) List(ctx context.Context) (auths Group, err error) {
	defer mon.Task()(&ctx)(&err)
	err = authDB.db.Iterate(ctx, storage.IterateOptions{
		Recurse: true,
	}, func(ctx context.Context, iterator storage.Iterator) error {
		var listErrs errs.Group
		var listItem storage.ListItem
		for iterator.Next(ctx, &listItem) {
			var nextAuths Group
			if err := nextAuths.Unmarshal(listItem.Value); err != nil {
				listErrs.Add(err)
			}
			auths = append(auths, nextAuths...)
		}
		return listErrs.Err()
	})
	return auths, err
}

// Claim marks an authorization as claimed and records claim information.
func (authDB *DB) Claim(ctx context.Context, opts *ClaimOpts) (err error) {
	defer mon.Task()(&ctx)(&err)
	now := time.Now()
	reqTime := time.Unix(opts.Req.Timestamp, 0)
	if (now.Sub(reqTime) > MaxClockSkew) ||
		(reqTime.Sub(now) > MaxClockSkew) {
		return Error.New("claim timestamp is outside of max delay window: %d", opts.Req.Timestamp)
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
		return Error.New("difficulty must be greater than: %d", opts.MinDifficulty)
	}

	token, err := ParseToken(opts.Req.AuthToken)
	if err != nil {
		return err
	}

	auths, err := authDB.Get(ctx, token.UserID)
	if err != nil {
		return err
	}

	for i, auth := range auths {
		if auth.Token.Equal(token) {
			if auth.Claim != nil {
				return Error.New("authorization has already been claimed: %s", auth.String())
			}

			auths[i] = &Authorization{
				Token: auth.Token,
				Claim: &Claim{
					Timestamp:        now.Unix(),
					Addr:             opts.Peer.Addr.String(),
					Identity:         ident,
					SignedChainBytes: opts.ChainBytes,
				},
			}
			if err := authDB.put(ctx, token.UserID, auths); err != nil {
				return err
			}
			break
		}
	}

	mon.Meter("authorization_claim").Mark(1)
	return nil
}

// Unclaim removes a claim from an authorization.
func (authDB *DB) Unclaim(ctx context.Context, authToken string) (err error) {
	defer mon.Task()(&ctx)(&err)
	token, err := ParseToken(authToken)
	if err != nil {
		return err
	}

	auths, err := authDB.Get(ctx, token.UserID)
	if err != nil {
		return err
	}

	for i, auth := range auths {
		if auth.Token.Equal(token) {
			auths[i].Claim = nil
			return authDB.put(ctx, token.UserID, auths)
		}
	}
	mon.Meter("authorization_claim").Mark(1)
	return errs.New("token not found in authorizations DB")
}

func (authDB *DB) add(ctx context.Context, userID string, newAuths Group) (err error) {
	defer mon.Task()(&ctx, userID)(&err)

	auths, err := authDB.Get(ctx, userID)
	if err != nil {
		return err
	}

	auths = append(auths, newAuths...)
	return authDB.put(ctx, userID, auths)
}

func (authDB *DB) put(ctx context.Context, userID string, auths Group) (err error) {
	defer mon.Task()(&ctx, userID)(&err)

	authsBytes, err := auths.Marshal()
	if err != nil {
		return ErrDB.Wrap(err)
	}

	if err := authDB.db.Put(ctx, storage.Key(userID), authsBytes); err != nil {
		return ErrDB.Wrap(err)
	}
	return nil
}
