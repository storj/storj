// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcstatus

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
)

func TestLoggingSanitizer_Error(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	logPath := ctx.File("log")
	logFile, err := os.Create(logPath)
	require.NoError(t, err)
	defer ctx.Check(logFile.Close)

	wrapper := errs.Class("wrapper class")
	internalClass := errs.Class("internal class")
	internalErr := internalClass.New("internal error")
	exposedClass := errs.Class("exposed class")
	exposedErr := exposedClass.New("exposed error")
	msg := "message"

	testLogConfig := zap.NewDevelopmentConfig()
	testLogConfig.OutputPaths = []string{logPath}
	testLog, err := testLogConfig.Build()
	require.NoError(t, err)

	sanitizer := NewLoggingSanitizer(&wrapper, testLog, &internalClass)

	{
		t.Log("Exposed error")
		sanitizedErr := sanitizer.Error(msg, exposedErr)
		if assert.Error(t, sanitizedErr) {
			assert.Contains(t, sanitizedErr.Error(), exposedClass)
		}

		logData, err := ioutil.ReadAll(logFile)
		require.NoError(t, err)

		logStr := string(logData)
		assert.Contains(t, logStr, msg)
		assert.Contains(t, logStr, fmt.Sprintf(`"error": "%s: %s"`, wrapper, exposedErr))
	}

	{
		t.Log("Internal error")
		sanitizedErr := sanitizer.Error(msg, internalErr)
		if assert.Error(t, sanitizedErr) {
			assert.NotContains(t, sanitizedErr.Error(), internalClass)
			assert.Contains(t, sanitizedErr.Error(), wrapper.New(msg).Error())
		}

		logData, err := ioutil.ReadAll(logFile)
		require.NoError(t, err)

		logStr := string(logData)
		assert.Contains(t, logStr, msg)
		assert.Contains(t, logStr, fmt.Sprintf(`"error": "%s: %s"`, wrapper, internalErr))
	}
}
