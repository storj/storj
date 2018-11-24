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
		in       UserInfo
		out      error
	}{
		{
			testName: "Valid test 1",
			in: UserInfo{
				FirstName: "Bill",
				Password:  "123456",
				Email:     "abcdqe@yahoo.com",
				LastName:  "Gate",
			},
			out: nil,
		},
		{
			testName: "Valid test 2",
			in: UserInfo{
				FirstName: "Arnold",
				Password:  "1234561231231231231231231231231231231231231231231231231231231231231231231",
				Email:     "Arnold.Snowmilk@yahoo.com",
				LastName:  "Snowmilk",
			},
			out: nil,
		},
		{
			testName: "Valid test 3",
			in: UserInfo{
				FirstName: "Arnold",
				Password:  "12345612312312312312312",
				Email:     "email@gmail.com",
				LastName:  "Snowmilk",
			},
			out: nil,
		},
		{
			testName: "Invalid test 1",
			in: UserInfo{
				FirstName: "",
				Password:  "123456123123123123123123123123123123123123",
				Email:     "Arnold.Snowmilk@yahoo.com",
				LastName:  "Snowmilk",
			},
			out: ErrMissingField("first name"),
		},
		{
			testName: "Invalid test 2",
			in: UserInfo{
				FirstName: "qwe",
				Password:  "123",
				Email:     "Arnold.Snowmilk@yahoo.com",
				LastName:  "Snowmilk",
			},
			out: ErrInvalidField("password"),
		},
		{
			testName: "Invalid test 3",
			in: UserInfo{
				FirstName: "qwe",
				Password:  "12334",
				Email:     "Arnold.Snowmilk@yahoo.com",
				LastName:  "Snowmilk",
			},
			out: ErrInvalidField("password"),
		},
		{
			testName: "Invalid test 4",
			in: UserInfo{
				FirstName: "",
				Password:  "",
				Email:     "Arnold.Snowmilk@yahoo.com",
				LastName:  "",
			},
			out: ErrMissingField("first name"),
		},
		{
			testName: "Invalid test 6",
			in: UserInfo{
				FirstName: "qweqwe",
				Password:  "qweqw123123",
				Email:     "",
				LastName:  "qweqweqwe",
			},
			out: ErrMissingField("email"),
		},
		{
			testName: "Invalid test 6",
			in: UserInfo{
				FirstName: "qweqwe",
				Password:  "qweqw123123",
				Email:     "qweqwe",
				LastName:  "qweqweqwe",
			},
			out: ErrInvalidField("email"),
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			assert.Equal(t, c.out, c.in.IsValid())
		})
	}
}

func TestCheckEmailAddressSyntax(t *testing.T) {
	cases := []struct {
		testName string
		in       string
		out      bool
	}{
		{
			testName: "Valid test 1",

			in:  "abcdqe@yahoo.com",
			out: true,
		},
		{
			testName: "Valid test 2",
			in:       "Arnold.Snowmilk@yahoo.com",
			out:      true,
		},
		{
			testName: "Valid test 3",
			in:       "email@gmail.com",
			out:      true,
		},
		{
			testName: "Valid test 4",
			in:       "email.email@gmail.com",
			out:      true,
		},
		{
			testName: "Valid test 5",
			in:       "email+extra@example.com",
			out:      true,
		},
		{
			testName: "Valid test 6",
			in:       "EMAIL@aol.co.uk",
			out:      true,
		},
		{
			testName: "Valid test 7",
			in:       "EMAIL+EXTRA@aol.co.uk",
			out:      true,
		},
		{
			testName: "Valid test 8",
			in:       "example-indeed@strange-example.com",
			out:      true,
		},
		{
			testName: "Valid test 9",
			in:       "other.email-with-hyphen@example.com",
			out:      true,
		},
		{
			testName: "Invalid test 1",
			in:       "",
			out:      false,
		},
		{
			testName: "Invalid test 2",
			in:       "some_invalid_email@,,,",
			out:      false,
		},
		{
			testName: "Invalid test 3",
			in:       "a\"b(c)d,e:f;g<h>i[j\\k]l@example.com",
			out:      false,
		},
		{
			testName: "Invalid test 4",
			in:       "email@",
			out:      false,
		},
		{
			testName: "Invalid test 5",
			in:       "email@x",
			out:      false,
		},
		{
			testName: "Invalid test 6",
			in:       "email@@example.com",
			out:      false,
		},
		{
			testName: "Invalid test 7",
			in:       "email...@example.com",
			out:      false,
		},
		{
			testName: "Invalid test 8",
			in:       "email..test@example.com",
			out:      false,
		},
		{
			testName: "Invalid test 9",
			in:       ".email..test.@example.com",
			out:      false,
		},
		{
			testName: "Invalid test 10",
			in:       "email@at@example.com",
			out:      false,
		},
		{
			testName: "Invalid test 11",
			in:       "some whitespace@example.com",
			out:      false,
		},
		{
			testName: "Invalid test 12",
			in:       "email@whitespace example.com",
			out:      false,
		},
		{
			testName: "Invalid test 13",
			in:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa@example.com",
			out:      false,
		},
		{
			testName: "Invalid test 14",
			in:       "email@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com",
			out:      false,
		},
		{
			testName: "Invalid test 20",
			in:       "just\"not\"right@example.com",
			out:      false,
		},
		{
			testName: "Invalid test 21",
			in:       "john..doe@example.com",
			out:      false,
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			assert.Equal(t, c.out, checkEmailAddressSyntax(c.in))
		})
	}
}
