// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
)

func saveInitialConfig(ctx context.Context, ex ulext.External, interactiveFlag bool, analyticsFlag *bool) error {
	var analyticsEnabled bool
	if analyticsFlag != nil {
		analyticsEnabled = *analyticsFlag
	} else {
		if interactiveFlag {
			answer, err := ex.PromptInput(ctx, `With your permission, Storj can `+
				`automatically collect analytics information from your uplink CLI to `+
				`help improve the quality and performance of our products. This `+
				`information is sent only with your consent and is submitted `+
				`anonymously to Storj Labs: (y/n)`)
			if err != nil {
				return errs.Wrap(err)
			}
			answer = strings.ToLower(answer)
			analyticsEnabled = answer == "y" || answer == "yes"
		} else {
			analyticsEnabled = false
		}
	}

	values := make(map[string]string)

	if analyticsEnabled {
		values["analytics.enabled"] = "true"
	} else {
		values["metrics.addr"] = ""
		values["analytics.enabled"] = "false"
	}

	return ex.SaveConfig(values)
}
