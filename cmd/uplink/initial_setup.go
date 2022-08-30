// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
)

func saveInitialConfig(ctx context.Context, ex ulext.External) error {
	answer, err := ex.PromptInput(ctx, `With your permission, Storj can `+
		`automatically collect analytics information from your uplink CLI to `+
		`help improve the quality and performance of our products. This `+
		`information is sent only with your consent and is submitted `+
		`anonymously to Storj Labs: (y/n)`)
	if err != nil {
		return errs.Wrap(err)
	}
	answer = strings.ToLower(answer)

	values := make(map[string]string)
	if answer != "y" && answer != "yes" {
		values["metrics.addr"] = ""
	}

	return ex.SaveConfig(values)
}
