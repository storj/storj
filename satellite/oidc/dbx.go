// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package oidc

import (
	"context"
	"database/sql"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type clientsDBX struct {
	db *dbx.DB
}

// Get returns the OAuthClient associated with the provided id.
func (clients *clientsDBX) Get(ctx context.Context, id uuid.UUID) (OAuthClient, error) {
	oauthClient, err := clients.db.Get_OauthClient_By_Id(ctx, dbx.OauthClient_Id(id.Bytes()))
	if err != nil {
		return OAuthClient{}, err
	}

	userID, err := uuid.FromBytes(oauthClient.UserId)
	if err != nil {
		return OAuthClient{}, err
	}

	client := OAuthClient{
		ID:          id,
		Secret:      oauthClient.EncryptedSecret,
		UserID:      userID,
		RedirectURL: oauthClient.RedirectUrl,
		AppName:     oauthClient.AppName,
		AppLogoURL:  oauthClient.AppLogoUrl,
	}

	return client, nil
}

// Create creates a new OAuthClient.
func (clients *clientsDBX) Create(ctx context.Context, client OAuthClient) (err error) {
	defer mon.Task()(&ctx)(&err)

	return clients.db.CreateNoReturn_OauthClient(ctx,
		dbx.OauthClient_Id(client.ID.Bytes()), dbx.OauthClient_EncryptedSecret(client.Secret),
		dbx.OauthClient_RedirectUrl(client.RedirectURL), dbx.OauthClient_UserId(client.UserID.Bytes()),
		dbx.OauthClient_AppName(client.AppName), dbx.OauthClient_AppLogoUrl(client.AppLogoURL))
}

// Update modifies information for the provided OAuthClient.
func (clients *clientsDBX) Update(ctx context.Context, client OAuthClient) (err error) {
	defer mon.Task()(&ctx)(&err)

	if client.RedirectURL == "" && client.Secret == nil {
		return nil
	}

	update := dbx.OauthClient_Update_Fields{}

	if client.RedirectURL != "" {
		update.RedirectUrl = dbx.OauthClient_RedirectUrl(client.RedirectURL)
	}

	if client.Secret != nil {
		update.EncryptedSecret = dbx.OauthClient_EncryptedSecret(client.Secret)
	}

	return clients.db.UpdateNoReturn_OauthClient_By_Id(ctx, dbx.OauthClient_Id(client.ID.Bytes()), update)
}

func (clients *clientsDBX) Delete(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = clients.db.Delete_OauthClient_By_Id(ctx, dbx.OauthClient_Id(id.Bytes()))
	return err
}

type codesDBX struct {
	db *dbx.DB
}

func (o *codesDBX) Get(ctx context.Context, code string) (oauthCode OAuthCode, err error) {
	defer mon.Task()(&ctx)(&err)

	dbCode, err := o.db.Get_OauthCode_By_Code_And_ClaimedAt_Is_Null(ctx, dbx.OauthCode_Code(code))
	if err != nil {
		return oauthCode, err
	}

	clientID, err := uuid.FromBytes(dbCode.ClientId)
	if err != nil {
		return oauthCode, err
	}

	userID, err := uuid.FromBytes(dbCode.UserId)
	if err != nil {
		return oauthCode, err
	}

	if time.Now().After(dbCode.ExpiresAt) {
		return oauthCode, sql.ErrNoRows
	}

	oauthCode.ClientID = clientID
	oauthCode.UserID = userID
	oauthCode.Scope = dbCode.Scope
	oauthCode.RedirectURL = dbCode.RedirectUrl
	oauthCode.Challenge = dbCode.Challenge
	oauthCode.ChallengeMethod = dbCode.ChallengeMethod
	oauthCode.Code = dbCode.Code
	oauthCode.CreatedAt = dbCode.CreatedAt
	oauthCode.ExpiresAt = dbCode.ExpiresAt
	oauthCode.ClaimedAt = dbCode.ClaimedAt

	return oauthCode, nil
}

func (o *codesDBX) Create(ctx context.Context, code OAuthCode) (err error) {
	defer mon.Task()(&ctx)(&err)

	return o.db.CreateNoReturn_OauthCode(ctx, dbx.OauthCode_ClientId(code.ClientID.Bytes()),
		dbx.OauthCode_UserId(code.UserID.Bytes()), dbx.OauthCode_Scope(code.Scope),
		dbx.OauthCode_RedirectUrl(code.RedirectURL), dbx.OauthCode_Challenge(code.Challenge),
		dbx.OauthCode_ChallengeMethod(code.ChallengeMethod), dbx.OauthCode_Code(code.Code),
		dbx.OauthCode_CreatedAt(code.CreatedAt), dbx.OauthCode_ExpiresAt(code.ExpiresAt), dbx.OauthCode_Create_Fields{})
}

func (o *codesDBX) Claim(ctx context.Context, code string) (err error) {
	defer mon.Task()(&ctx)(&err)

	return o.db.UpdateNoReturn_OauthCode_By_Code_And_ClaimedAt_Is_Null(ctx, dbx.OauthCode_Code(code), dbx.OauthCode_Update_Fields{
		ClaimedAt: dbx.OauthCode_ClaimedAt(time.Now()),
	})
}

type tokensDBX struct {
	db *dbx.DB
}

func (o *tokensDBX) Get(ctx context.Context, kind OAuthTokenKind, token string) (oauthToken OAuthToken, err error) {
	defer mon.Task()(&ctx)(&err)

	dbToken, err := o.db.Get_OauthToken_By_Kind_And_Token(ctx, dbx.OauthToken_Kind(int(kind)),
		dbx.OauthToken_Token([]byte(token)))

	if err != nil {
		return oauthToken, err
	}

	clientID, err := uuid.FromBytes(dbToken.ClientId)
	if err != nil {
		return oauthToken, err
	}

	userID, err := uuid.FromBytes(dbToken.UserId)
	if err != nil {
		return oauthToken, err
	}

	if time.Now().After(dbToken.ExpiresAt) {
		return oauthToken, sql.ErrNoRows
	}

	oauthToken.ClientID = clientID
	oauthToken.UserID = userID
	oauthToken.Scope = dbToken.Scope
	oauthToken.Kind = OAuthTokenKind(dbToken.Kind)
	oauthToken.Token = token
	oauthToken.CreatedAt = dbToken.CreatedAt
	oauthToken.ExpiresAt = dbToken.ExpiresAt

	return oauthToken, nil
}

func (o *tokensDBX) Create(ctx context.Context, token OAuthToken) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = o.db.CreateNoReturn_OauthToken(ctx, dbx.OauthToken_ClientId(token.ClientID.Bytes()),
		dbx.OauthToken_UserId(token.UserID.Bytes()), dbx.OauthToken_Scope(token.Scope),
		dbx.OauthToken_Kind(int(token.Kind)), dbx.OauthToken_Token([]byte(token.Token)),
		dbx.OauthToken_CreatedAt(token.CreatedAt), dbx.OauthToken_ExpiresAt(token.ExpiresAt))

	// ignore duplicate key errors as they're somewhat expected
	if err != nil && dbx.IsConstraintError(err) {
		return nil
	}
	return err
}

// RevokeRESTTokenV0 revokes a v0 REST token by setting its expires_at time to zero.
func (o *tokensDBX) RevokeRESTTokenV0(ctx context.Context, token string) (err error) {
	defer mon.Task()(&ctx)(&err)

	return o.db.UpdateNoReturn_OauthToken_By_Token_And_Kind(ctx, dbx.OauthToken_Token([]byte(token)),
		dbx.OauthToken_Kind(int(KindRESTTokenV0)),
		dbx.OauthToken_Update_Fields{
			ExpiresAt: dbx.OauthToken_ExpiresAt(time.Time{}),
		})
}
