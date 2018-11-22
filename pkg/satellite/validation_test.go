// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationUserInfo(t *testing.T) {
	cases := []struct {
		testName string
		testFunc func()
	}{
		{
			testName: "Valid test 1",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "Bill",
					Password:  "123456",
					Email:     "abcdqe@yahoo.com",
					LastName:  "Gate",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, true, isValid)
			},
		},
		{
			testName: "Valid test 2",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "Arnold",
					Password:  "1234561231231231231231231231231231231231231231231231231231231231231231231",
					Email:     "Arnold.Snowmilk@yahoo.com",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, true, isValid)
			},
		},
		{
			testName: "Valid test 3",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "Arnold",
					Password:  "12345612312312312312312",
					Email:     "email@gmail.com",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, true, isValid)
			},
		},
		{
			testName: "Valid test 4",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "Arnold",
					Password:  "1234561231231231231231231231231",
					Email:     "email.email@gmail.com",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, true, isValid)
			},
		},
		{
			testName: "Valid test 5",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "Arnold",
					Password:  "12345612312312312312312312312312312",
					Email:     "email+extra@example.com",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, true, isValid)
			},
		},
		{
			testName: "Valid test 6",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "Arnold",
					Password:  "123456123123123123123123123123123123",
					Email:     "EMAIL@aol.co.uk",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, true, isValid)
			},
		},
		{
			testName: "Valid test 7",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "Arnold",
					Password:  "123456123123123123123123123123123123123",
					Email:     "EMAIL+EXTRA@aol.co.uk",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, true, isValid)
			},
		},
		{
			testName: "Valid test 8",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "Arnold",
					Password:  "123456123123123123123123123123123123123",
					Email:     "example-indeed@strange-example.com",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, true, isValid)
			},
		},
		{
			testName: "Valid test 9",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "Arnold",
					Password:  "123456123123123123123123123123123123123",
					Email:     "other.email-with-hyphen@example.com",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, true, isValid)
			},
		},
		{
			testName: "Invalid test 1",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "",
					Password:  "123456123123123123123123123123123123123123",
					Email:     "Arnold.Snowmilk@yahoo.com",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 2",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qwe",
					Password:  "123",
					Email:     "Arnold.Snowmilk@yahoo.com",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 3",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qwe",
					Password:  "12334",
					Email:     "Arnold.Snowmilk@yahoo.com",
					LastName:  "Snowmilk",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 4",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qwe",
					Password:  "1234567",
					Email:     "Arnold.Snowmilk@yahoo.com",
					LastName:  "",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 5",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "",
					Password:  "",
					Email:     "Arnold.Snowmilk@yahoo.com",
					LastName:  "",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 6",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 7",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "some_invalid_email@,,,",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 8",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "a\"b(c)d,e:f;g<h>i[j\\k]l@example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 9",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "email@",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 10",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "email@x",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 11",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "email@@example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 12",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "email...@example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 13",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "email..test@example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 14",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     ".email..test.@example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 15",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "email@at@example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 16",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "some whitespace@example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 17",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "email@whitespace example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 18",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa@example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 19",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "email@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 20",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "just\"not\"right@example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
		{
			testName: "Invalid test 21",

			testFunc: func() {
				userInfo := UserInfo{
					FirstName: "qweqwe",
					Password:  "qweqw123123",
					Email:     "john..doe@example.com",
					LastName:  "qweqweqwe",
				}

				isValid := userInfo.IsValid()

				assert.Equal(t, false, isValid)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.testFunc()
		})
	}
}
