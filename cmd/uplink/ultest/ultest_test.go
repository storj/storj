// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/storj/cmd/uplink/ultest"
)

func TestPromptResponder(t *testing.T) {
	const (
		commandName         = "command"
		expectedValue       = "my-value"
		expectedSecretValue = "my-secret-value"
	)

	responder := func(ctx context.Context, prompt string) (response string, err error) {
		switch prompt {
		case "Value:":
			return expectedValue, nil
		case "Secret Value:":
			return expectedSecretValue, nil
		}
		return "", errs.New("unknown prompt %q", prompt)
	}

	state := ultest.Setup(func(cmds clingy.Commands, ex ulext.External) {
		cmds.New(commandName, "", &testCommand{ex: ex})
	}, ultest.WithPromptResponder(responder))

	state.Succeed(t, commandName).RequireStdout(t, fmt.Sprintf("%s\n%s\n", expectedValue, expectedSecretValue))
}

type testCommand struct {
	ex ulext.External
}

func (cmd *testCommand) Setup(clingy.Parameters) {}

func (cmd *testCommand) Execute(ctx context.Context) (err error) {
	response, err := cmd.ex.PromptInput(ctx, "Value:")
	if err != nil {
		return errs.Wrap(err)
	}

	stdout := clingy.Stdout(ctx)
	if _, err := fmt.Fprintln(stdout, response); err != nil {
		return errs.Wrap(err)
	}

	response, err = cmd.ex.PromptSecret(ctx, "Secret Value:")
	if err != nil {
		return errs.Wrap(err)
	}

	if _, err := fmt.Fprintln(stdout, response); err != nil {
		return errs.Wrap(err)
	}

	return nil
}
