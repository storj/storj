// Copyright (C) 2018 Storj Labs, Inc.
//
// This file is part of the Storj client library.
//
// The Storj client library is free software: you can redistribute it and/or
// modify it under the terms of the GNU Lesser General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// The Storj client library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with The Storj client library.  If not, see
// <http://www.gnu.org/licenses/>.

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

var unmarshalTests = []struct {
	raw       string
	errString string
}{
	{"", "some error"},
	{"{", "some error"}, // syntax error
	{"{}", "some error"},
	{`{"info":{}}`, "some error"},
	{fmt.Sprintf(`{"info":{description":"%s","version":"%s"},"host":"%s"}`,
		mockDescription, mockVersion, mockHost), "some error"},
	{fmt.Sprintf(`{"info":{"title":"%s","version":"%s"},"host":"%s"}`,
		mockTitle, mockVersion, mockHost), "some error"},
	{fmt.Sprintf(`{"info":{"title":"%s","description":"%s"},"host":"%s"}`,
		mockTitle, mockDescription, mockHost), "some error"},
	{fmt.Sprintf(`{"info":{"title":"%s","description":"%s","version":"%s"}}`,
		mockTitle, mockDescription, mockVersion), "some error"},
	{fmt.Sprintf(`{"info":{"title":"%s","description":"%s","version":"%s"},"host":"%s"}`,
		mockTitle, mockDescription, mockVersion, mockHost), ""},
}

func TestUnmarshalJSON(t *testing.T) {
	for i, tt := range []struct {
		raw       string
		errString string
	}{
		{"", "unexpected end of JSON input"},
		{"{", "unexpected end of JSON input"},
		{"{}", "Missing info element in JSON response"},
		{`{"info":{}}`, "Missing title element in JSON response"},
		{fmt.Sprintf(`{"info":{"description":"%s","version":"%s"},"host":"%s"}`,
			mockDescription, mockVersion, mockHost),
			"Missing title element in JSON response"},
		{fmt.Sprintf(`{"info":{"title":"%s","version":"%s"},"host":"%s"}`,
			mockTitle, mockVersion, mockHost),
			"Missing description element in JSON response"},
		{fmt.Sprintf(`{"info":{"title":"%s","description":"%s"},"host":"%s"}`,
			mockTitle, mockDescription, mockHost),
			"Missing version element in JSON response"},
		{fmt.Sprintf(`{"info":{"title":"%s","description":"%s","version":"%s"}}`,
			mockTitle, mockDescription, mockVersion),
			"Missing host element in JSON response"},
		{fmt.Sprintf(`{"info":{"title":"%s","description":"%s","version":"%s"},"host":"%s"}`,
			mockTitle, mockDescription, mockVersion, mockHost), ""},
	} {
		var info Info
		err := json.Unmarshal([]byte(tt.raw), &info)
		errTag := fmt.Sprintf("Test case #%d", i)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		if assert.NoError(t, err, errTag) {
			checkInfo(info, t, errTag)
		}
	}
}

func TestGetInfo(t *testing.T) {
	for i, tt := range []struct {
		env       Env
		errString string
	}{
		{NewMockNoAuthEnv(), ""},
		{Env{URL: mockBridgeURL + "/info"}, "Unexpected response code: 404"},
	} {
		info, err := GetInfo(tt.env)
		errTag := fmt.Sprintf("Test case #%d", i)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		if assert.NoError(t, err, errTag) {
			checkInfo(info, t, errTag)
		}
	}
}

func checkInfo(info Info, t *testing.T, errTag string) {
	assert.Equal(t, mockTitle, info.Title, errTag)
	assert.Equal(t, mockDescription, info.Description, errTag)
	assert.Equal(t, mockVersion, info.Version, errTag)
	assert.Equal(t, mockHost, info.Host, errTag)
}
