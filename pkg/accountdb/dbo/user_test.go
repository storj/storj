// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package dbo

import (
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/accountdb/dbx"
	"testing"
	"time"
)

func TestUserDboFromDbx(t *testing.T) {

	cases := []struct {
		testName, address string
		testFunc          func()
	}{
		{
			testName: "can't create dbo from nil dbx model",
			testFunc: func() {
				dboUser := User{}

				user, err := dboUser.FromDbx(nil)

				assert.Nil(t, user)
				assert.NotNil(t, err)
				assert.Error(t, err)
			},
		},
		{
			testName: "can't create dbo from dbx model with invalid Id",
			testFunc: func() {
				dbxUser := dbx.User{
					"qweqwe",
					"FirstName",
					"LastName",
					"email@ukr.net",
					"ihqerfgnu238723huagsd",
					time.Now(),
				}

				dboUser := User{}

				user, err := dboUser.FromDbx(&dbxUser)

				assert.Nil(t, user)
				assert.NotNil(t, err)
				assert.Error(t, err)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) { c.testFunc() })
	}

}
