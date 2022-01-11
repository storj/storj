// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type oauthClients struct {
	methods dbx.Methods
	db      *satelliteDB
}

// Get returns the OAuthClient associated with the provided id.
func (clients *oauthClients) Get(ctx context.Context, id uuid.UUID) (console.OAuthClient, error) {
	oauthClient, err := clients.db.Get_OauthClient_By_Id(ctx, dbx.OauthClient_Id(id.Bytes()))
	if err != nil {
		return console.OAuthClient{}, err
	}

	userID, err := uuid.FromBytes(oauthClient.UserId)
	if err != nil {
		return console.OAuthClient{}, err
	}

	client := console.OAuthClient{
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
func (clients *oauthClients) Create(ctx context.Context, client console.OAuthClient) error {
	_, err := clients.db.Create_OauthClient(ctx,
		dbx.OauthClient_Id(client.ID.Bytes()), dbx.OauthClient_EncryptedSecret(client.Secret),
		dbx.OauthClient_RedirectUrl(client.RedirectURL), dbx.OauthClient_UserId(client.UserID.Bytes()),
		dbx.OauthClient_AppName(client.AppName), dbx.OauthClient_AppLogoUrl(client.AppLogoURL))

	return err
}

// Update modifies information for the provided OAuthClient.
func (clients *oauthClients) Update(ctx context.Context, client console.OAuthClient) error {
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

	err := clients.db.UpdateNoReturn_OauthClient_By_Id(ctx, dbx.OauthClient_Id(client.ID.Bytes()), update)
	return err
}

func (clients *oauthClients) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := clients.db.Delete_OauthClient_By_Id(ctx, dbx.OauthClient_Id(id.Bytes()))
	return err
}
