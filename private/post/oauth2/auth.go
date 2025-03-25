// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	mon = monkit.Package()
)

// Auth is XOAUTH2 implementation of smtp.Auth interface.
type Auth struct {
	UserEmail string

	Storage *TokenStore
}

// Start returns proto and auth credentials for first auth msg.
func (auth *Auth) Start(server *smtp.ServerInfo) (proto string, toServer []byte, err error) {
	ctx := context.TODO()
	defer mon.Task()(&ctx)(&err)
	if !server.TLS {
		return "", nil, errs.New("unencrypted connection")
	}

	token, err := auth.Storage.Token(ctx)
	if err != nil {
		return "", nil, err
	}

	format := fmt.Sprintf("user=%s\x01auth=%s %s\x01\x01", auth.UserEmail, token.Type, token.AccessToken)
	return "XOAUTH2", []byte(format), nil
}

// Next sends empty response to solve SASL challenge if response code is 334.
func (auth *Auth) Next(fromServer []byte, more bool) (toServer []byte, err error) {
	if more {
		return []byte{}, nil
	}
	return nil, nil
}

// Token represents OAuth2 token.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Type         string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}

// Credentials represents OAuth2 credentials.
type Credentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	TokenURI     string `json:"token_uri"`
}

// TokenStore is a thread safe storage for OAuth2 token and credentials.
type TokenStore struct {
	mu    sync.Mutex
	token Token
	creds Credentials
}

// NewTokenStore creates new instance of token storage.
func NewTokenStore(creds Credentials, token Token) *TokenStore {
	return &TokenStore{
		token: token,
		creds: creds,
	}
}

// Token retrieves token in a thread safe way and refreshes it if needed.
func (s *TokenStore) Token(ctx context.Context) (_ *Token, err error) {
	defer mon.Task()(&ctx)(&err)
	s.mu.Lock()
	defer s.mu.Unlock()

	token := new(Token)
	if s.token.Expiry.Before(time.Now()) {
		var err error
		token, err = RefreshToken(ctx, s.creds, s.token.RefreshToken)
		if err != nil {
			return nil, err
		}
		s.token = *token
	}

	*token = s.token
	return token, nil
}

// RefreshToken is a helper method that refreshes token with given credentials and OUATH2 refresh token.
func RefreshToken(ctx context.Context, creds Credentials, refreshToken string) (_ *Token, err error) {
	defer mon.Task()(&ctx)(&err)

	values := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, creds.TokenURI, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(creds.ClientID), url.QueryEscape(creds.ClientSecret))

	client := http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	// handle google expires_in field value
	var t struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Type         string `json:"token_type"`
		Expires      int64  `json:"expires_in"`
	}
	err = json.NewDecoder(resp.Body).Decode(&t)
	if err != nil {
		return nil, err
	}

	if t.AccessToken == "" {
		return nil, errs.New("no access token were granted")
	}

	if t.RefreshToken == "" {
		t.RefreshToken = refreshToken
	}

	return &Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		Type:         t.Type,
		Expiry:       time.Now().Add(time.Duration(t.Expires * int64(time.Second))),
	}, nil
}
