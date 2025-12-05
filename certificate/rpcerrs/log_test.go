// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcerrs_test

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/testcontext"
	"storj.io/storj/certificate/rpcerrs"
)

func TestLoggingSanitizer_Error(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	logPath := ctx.File("log")
	logFile, err := os.Create(logPath)
	require.NoError(t, err)
	defer ctx.Check(logFile.Close)

	wrapper := errs.Class("wrapper class")
	unauthenticatedClass := errs.Class("unauthorized class")
	notFoundClass := errs.Class("not found class")
	internalClass := errs.Class("internal class")
	internalErr := internalClass.New("internal error")
	msg := "message"
	statusMap := rpcerrs.StatusMap{
		&unauthenticatedClass: rpcstatus.Unauthenticated,
		&notFoundClass:        rpcstatus.NotFound,
	}

	testLogConfig := zap.NewDevelopmentConfig()
	testLogConfig.OutputPaths = []string{logPath}
	testLog, err := testLogConfig.Build()
	require.NoError(t, err)

	scenarios := []struct {
		name    string
		wrapper *errs.Class
		log     *zap.Logger
	}{
		{
			"with wrapper and log",
			&wrapper,
			testLog,
		},
		{
			"with wrapper, no log",
			&wrapper,
			nil,
		},
		{
			"with log, no wrapper",
			nil,
			testLog,
		},
		{
			"no wrapper or log",
			nil,
			nil,
		},
	}

	for _, s := range scenarios {
		s := s
		t.Run(s.name, func(t *testing.T) {
			sanitizer := rpcerrs.NewLog(s.wrapper, s.log, statusMap)

			t.Log("exposed errors")
			for errClass, code := range statusMap {
				errInstance := errClass.New("%s", strings.Replace(string(*errClass), "class", "error", 1))

				sanitizedErr := sanitizer.Error(msg, errInstance)
				require.Error(t, sanitizedErr)
				require.Equal(t, code, rpcstatus.Code(sanitizedErr))
				require.Contains(t, sanitizedErr.Error(), *errClass)
				if s.wrapper == nil {
					require.Contains(t, sanitizedErr.Error(), errInstance.Error())
				} else {
					require.Contains(t, sanitizedErr.Error(), wrapper.Wrap(errInstance).Error())
				}

				if s.log != nil {
					logData, err := io.ReadAll(logFile)
					require.NoError(t, err)

					logStr := string(logData)
					require.Contains(t, logStr, msg)
					if s.wrapper == nil {
						require.Contains(t, logStr, fmt.Sprintf(`"error": "%s"`, errInstance))
					} else {
						require.Contains(t, logStr, fmt.Sprintf(`"error": "%s: %s"`, wrapper, errInstance))
					}
				}
			}

			t.Log("internal error")
			sanitizedErr := sanitizer.Error(msg, internalErr)
			require.Error(t, sanitizedErr)
			require.Equal(t, rpcstatus.Internal, rpcstatus.Code(sanitizedErr))
			require.NotContains(t, sanitizedErr.Error(), internalClass)
			if s.wrapper == nil {
				require.Contains(t, sanitizedErr.Error(), msg)
			} else {
				require.Equal(t, wrapper.New("%v", msg).Error(), sanitizedErr.Error())
			}

			if s.log != nil {
				logData, err := io.ReadAll(logFile)
				require.NoError(t, err)

				logStr := string(logData)
				require.Contains(t, logStr, msg)
				if s.wrapper == nil {
					require.Contains(t, logStr, fmt.Sprintf(`"error": "%s"`, internalErr))
				} else {
					require.Contains(t, logStr, fmt.Sprintf(`"error": "%s: %s"`, wrapper, internalErr))
				}
			}
		})
	}
}
