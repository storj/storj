// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package gmail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"
)

// Auth is XOAUTH2 implementation of smtp.Auth interface for gmail
type Auth struct {
	UserEmail string

	Storage *TokenStore
}

// Start returns proto and auth credentials for first auth msg
func (auth *Auth) Start(server *smtp.ServerInfo) (proto string, toServer []byte, err error) {
	token, err := auth.Storage.Token()
	if err != nil {
		return "", nil, err
	}

	format := fmt.Sprintf("user=%s\001auth=%s %s\001\001", auth.UserEmail, token.Type, token.AccessToken)
	return "XOAUTH2", []byte(format), nil
}

// Next sends empty response to solve SASL challenge if response code is 334
func (auth *Auth) Next(fromServer []byte, more bool) (toServer []byte, err error) {
	if more {
		return make([]byte, 0), nil
	}

	return nil, nil
}

// Token represents OAUTH2 token
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Type         string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}

// Credentials represents OAUTH2 credentials
type Credentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	TokenURI     string `json:"token_uri"`
}

// TokenStore is a thread safe storage for OAUTH2 token and credentials
type TokenStore struct {
	mu    sync.Mutex
	token Token
	creds Credentials
}

// NewTokenStore creates new instance of token storage
func NewTokenStore(creds Credentials, token Token) *TokenStore {
	return &TokenStore{
		token: token,
		creds: creds,
	}
}

// Token retrieves token in a thread safe way and refreshes it if needed
func (s *TokenStore) Token() (*Token, error) {
	s.mu.Lock()
	token := new(Token)
	if s.token.Expiry.Before(time.Now()) {
		var err error
		token, err = RefreshToken(s.creds, s.token.RefreshToken)
		if err != nil {
			s.mu.Unlock()
			return nil, err
		}
		s.token = *token
	}

	*token = s.token
	s.mu.Unlock()
	return token, nil
}

// RefreshToken is a helper method that refreshes token with given credentials and OUATH2 refresh token
func RefreshToken(creds Credentials, refreshToken string) (*Token, error) {
	values := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequest("POST", creds.TokenURI, strings.NewReader(values.Encode()))
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
		err = errs.Combine(resp.Body.Close())
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// handle google expires_in field value
	var t struct {
		AccessToken  string        `json:"access_token"`
		RefreshToken string        `json:"refresh_token"`
		Type         string        `json:"token_type"`
		Expires      time.Duration `json:"expires_in"`
	}
	err = json.NewDecoder(bytes.NewReader(body)).Decode(&t)
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
		Expiry:       time.Now().Add(t.Expires * time.Second),
	}, nil
}

// CredsFromJSON is a helper method for parsing OAUTH2 google app credentials from raw json data
func CredsFromJSON(jsonData []byte) (*Credentials, error) {
	var j struct {
		Web       *Credentials `json:"web"`
		Installed *Credentials `json:"installed"`
	}

	if err := json.NewDecoder(bytes.NewReader(jsonData)).Decode(&j); err != nil {
		return nil, err
	}

	switch {
	case j.Web != nil:
		return j.Web, nil
	case j.Installed != nil:
		return j.Installed, nil
	default:
		return nil, errs.New("no credentials found")
	}
}
