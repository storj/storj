// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package valdiclient

import (
	"crypto/rsa"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pkcrypto"
)

var (
	mon = monkit.Package()

	// ErrPrivateKey is error type of private key.
	ErrPrivateKey = errs.Class("private key")

	// ErrAPIURL is error type of api url.
	ErrAPIURL = errs.Class("api url")
)

// Config holds configuration of Client.
type Config struct {
	APIBaseURL   string `help:"base url of valdi external API" releaseDefault:"https://api.valdi.ai" devDefault:"http://localhost:1234"`
	RSAKeyPath   string `help:"path to RSA private key for signing valdi requests" default:""`
	SignRequests bool   `help:"whether to sign valdi requests with valdi RSA key" releaseDefault:"true" devDefault:"false" hidden:"true"`
}

// Client makes requests to Valdi API.
type Client struct {
	log          *zap.Logger
	client       *http.Client
	baseURL      string
	privateKey   *rsa.PrivateKey
	signRequests bool
}

// ErrorMessage contains error message returned by Valdi.
type ErrorMessage struct {
	Detail string `json:"detail"`
}

// New creates a new valdi client.
func New(log *zap.Logger, httpClient *http.Client, config Config) (*Client, error) {
	err := verifyURL(config.APIBaseURL)
	if err != nil {
		return nil, err
	}

	var rsaKey *rsa.PrivateKey
	if config.SignRequests {
		keyPEM, err := os.ReadFile(config.RSAKeyPath)
		if err != nil {
			return nil, ErrPrivateKey.New("failed to read key file: %w", err)
		}

		key, err := pkcrypto.PrivateKeyFromPEM(keyPEM)
		if err != nil {
			return nil, ErrPrivateKey.New("failed to parse key data: %w", err)
		}

		var ok bool
		rsaKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, ErrPrivateKey.New("key is not RSA private key")
		}
		if rsaKey == nil {
			return nil, ErrPrivateKey.New("RSA key for signing valdi requests is nil")
		}
	}
	return &Client{
		log:          log,
		client:       httpClient,
		baseURL:      config.APIBaseURL,
		privateKey:   rsaKey,
		signRequests: config.SignRequests,
	}, nil
}

func verifyURL(apiBaseURL string) error {
	parsedURL, err := url.Parse(apiBaseURL)
	if err != nil {
		return ErrAPIURL.New("invalid APIBaseURL: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return ErrAPIURL.New("APIBaseURL must be http or https")
	}

	if parsedURL.Host == "" {
		return ErrAPIURL.New("APIBaseURL must have a host")
	}

	return nil
}

func (c *Client) createJWT(valdiEmail string) *jwt.Token {
	claims := jwt.MapClaims{
		"email":   valdiEmail,
		"purpose": "storj_user_action",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Minute).Unix(),
	}

	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
}
