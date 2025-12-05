// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package prompt implements asking input from command line.
package prompt

import (
	"fmt"
	"strings"

	"github.com/zeebo/errs"
)

// Error is the default error class for prompt package.
var Error = errs.Class("prompt")

// Confirm asks to confirm a question.
func Confirm(prompt string) (bool, error) {
	for {
		fmt.Print(prompt + " :")

		var response string
		_, err := fmt.Scanln(&response)
		if err != nil {
			return false, Error.Wrap(err)
		}
		response = strings.TrimSpace(response)

		switch strings.ToLower(response) {
		case "yes", "y", "true":
			return true, nil
		case "no", "n", "false":
			return false, nil
		}
	}
}
