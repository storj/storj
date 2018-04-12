// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	mockTitle       = "Storj Bridge"
	mockDescription = "Some description"
	mockVersion     = "1.2.3"
	mockHost        = "1.2.3.4"
)

func TestUnmarshalJSON(t *testing.T) {
	for i, tt := range []struct {
		json      string
		info      Info
		errString string
	}{
		{"", Info{}, "unexpected end of JSON input"},
		{"{", Info{}, "unexpected end of JSON input"},
		{"{}", Info{}, ""},
		{`{"info":{}}`, Info{}, ""},
		{`{"info":10}`, Info{}, ""},
		{`{"info":{"title":10,"description":10,"version":10},"host":10}`, Info{}, ""},
		{fmt.Sprintf(`{"info":{"description":"%s","version":"%s"},"host":"%s"}`,
			mockDescription, mockVersion, mockHost),
			Info{
				Description: mockDescription,
				Version:     mockVersion,
				Host:        mockHost,
			},
			""},
		{fmt.Sprintf(`{"info":{"title":"%s","version":"%s"},"host":"%s"}`,
			mockTitle, mockVersion, mockHost),
			Info{
				Title:   mockTitle,
				Version: mockVersion,
				Host:    mockHost,
			},
			""},
		{fmt.Sprintf(`{"info":{"title":"%s","description":"%s"},"host":"%s"}`,
			mockTitle, mockDescription, mockHost),
			Info{
				Title:       mockTitle,
				Description: mockDescription,
				Host:        mockHost,
			},
			""},
		{fmt.Sprintf(`{"info":{"title":"%s","description":"%s","version":"%s"}}`,
			mockTitle, mockDescription, mockVersion),
			Info{
				Title:       mockTitle,
				Description: mockDescription,
				Version:     mockVersion,
			},
			""},
		{fmt.Sprintf(`{"info":{"title":"%s","description":"%s","version":"%s"},"host":"%s"}`,
			mockTitle, mockDescription, mockVersion, mockHost),
			Info{
				Title:       mockTitle,
				Description: mockDescription,
				Version:     mockVersion,
				Host:        mockHost,
			},
			""},
	} {
		var info Info
		err := json.Unmarshal([]byte(tt.json), &info)
		errTag := fmt.Sprintf("Test case #%d", i)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, tt.info, info, errTag)
		}
	}
}

func TestGetInfo(t *testing.T) {
	for i, tt := range []struct {
		env       Env
		errString string
	}{
		{NewMockNoAuthEnv(), ""},
		{Env{URL: mockBridgeURL + "/info"}, "unexpected status code: 404"},
	} {
		info, err := GetInfo(tt.env)
		errTag := fmt.Sprintf("Test case #%d", i)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, mockTitle, info.Title, errTag)
			assert.Equal(t, mockDescription, info.Description, errTag)
			assert.Equal(t, mockVersion, info.Version, errTag)
			assert.Equal(t, mockHost, info.Host, errTag)
		}
	}
}
