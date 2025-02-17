// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package sso

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/spf13/pflag"
)

// Config is a configuration struct for SSO.
type Config struct {
	Enabled               bool                  `help:"whether SSO is enabled." default:"false"`
	OidcProviderInfos     OidcProviderInfos     `help:"semicolon-separated provider:client-id,client-secret,provider-url." default:""`
	EmailProviderMappings EmailProviderMappings `help:"semicolon-separated provider:email-regex as provided in oidc-provider-infos." default:""`
	MockSso               bool                  `help:"whether to mock SSO for testing purposes. This should never be true in production." default:"false" hidden:"true"`
	MockEmail             string                `help:"mock email for successful SSO auth for testing purposes." default:"" hidden:"true"`
}

// Ensure that OidcProviderInfos implements pflag.Value.
var _ pflag.Value = (*OidcProviderInfos)(nil)

// OidcProviderInfo contains the information needed to connect to an OIDC provider.
type OidcProviderInfo struct {
	ClientID     string
	ClientSecret string
	ProviderURL  url.URL
}

// OidcProviderInfos is a map of SSO providers to OIDC provider infos.
type OidcProviderInfos struct {
	Values map[string]OidcProviderInfo
}

// Type returns the type of the pflag.Value.
func (OidcProviderInfos) Type() string { return "sso.infos" }

func (si *OidcProviderInfos) String() string {
	var s strings.Builder
	i := 0
	for k, v := range si.Values {
		if i > 0 {
			s.WriteString(";")
		}
		_, _ = fmt.Fprintf(&s, "%s:%s,%s,%s", k, v.ClientID, v.ClientSecret, v.ProviderURL.String())
		i++
	}
	return s.String()
}

// Set OIDC provider infos to the parsed string.
func (si *OidcProviderInfos) Set(s string) error {
	keyInfosMap := make(map[string]OidcProviderInfo)
	for _, keyStr := range strings.Split(s, ";") {
		if keyStr == "" {
			continue
		}

		info := strings.Split(keyStr, ":")
		if len(info) < 2 {
			return Error.New("invalid string (expected format provider:client-id,client-secret,provider-url, got %s)", keyStr)
		}

		provider := strings.TrimSpace(info[0])
		if provider == "" {
			return Error.New("provider must not be empty")
		}
		if _, ok := keyInfosMap[provider]; ok {
			return Error.New("provider duplicate found. Provider must be unique: %s", provider)
		}

		valuesStr := strings.Replace(keyStr, provider+":", "", 1)
		values := strings.Split(valuesStr, ",")
		if len(values) != 3 {
			return Error.New("Invalid values (expected format client-id,client-secret,provider-url, got %s)", valuesStr)
		}

		providerUrl, err := url.Parse(strings.TrimSpace(values[2]))
		if err != nil {
			return Error.Wrap(err)
		}

		keyInfosMap[provider] = OidcProviderInfo{
			ClientID:     strings.TrimSpace(values[0]),
			ClientSecret: strings.TrimSpace(values[1]),
			ProviderURL:  *providerUrl,
		}
	}
	si.Values = keyInfosMap
	return nil
}

// Ensure that OidcProviderInfos implements pflag.Value.
var _ pflag.Value = (*EmailProviderMappings)(nil)

// EmailProviderMappings is a map of sso provider to email regex.
type EmailProviderMappings struct {
	Values map[string]regexp.Regexp
}

// Type returns the type of the pflag.Value.
func (EmailProviderMappings) Type() string { return "sso.email-provider-mappings" }

func (epm *EmailProviderMappings) String() string {
	var s strings.Builder
	i := 0
	for k, v := range epm.Values {
		if i > 0 {
			s.WriteString(";")
		}
		_, _ = fmt.Fprintf(&s, "%s:%s", k, v.String())
		i++
	}
	return s.String()
}

// Set email provider mappings to a provided parsed string.
func (epm *EmailProviderMappings) Set(s string) error {
	mappingsMap := make(map[string]regexp.Regexp)
	for _, keyStr := range strings.Split(s, ";") {
		if keyStr == "" {
			continue
		}

		info := strings.Split(keyStr, ":")
		if len(info) != 2 {
			return Error.New("invalid string (expected format provider:email-regex, got %s)", keyStr)
		}

		provider := strings.TrimSpace(info[0])
		if provider == "" {
			return Error.New("provider must not be empty")
		}
		if _, ok := mappingsMap[provider]; ok {
			return Error.New("provider duplicate found. Provider must be unique: %s", provider)
		}

		regexStr := strings.TrimSpace(info[1])
		emailSuffix := regexp.MustCompile(regexStr)
		if emailSuffix == nil {
			return Error.New("invalid email suffix regex: %s", regexStr)
		}

		mappingsMap[provider] = *emailSuffix
	}
	epm.Values = mappingsMap
	return nil
}
