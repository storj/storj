// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package lazyfilewalker

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestZapWrapper(t *testing.T) {
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	w := &zapWrapper{Log: observedLogger}

	t.Run("valid logs", func(t *testing.T) {
		_, err := io.WriteString(w, `{"L":"INFO","T":"2023-06-29T16:01:12.361Z","C":"internalcmd/used_space_filewalker.go:85","M":"Database started"}`)
		require.NoError(t, err)

		_, err = io.WriteString(w, `{"L":"INFO","T":"2023-06-29T16:01:12.361Z","C":"internalcmd/used_space_filewalker.go:90","M":"used-space-filewalker started"}`)
		require.NoError(t, err)

		_, err = io.WriteString(w, `{"L":"INFO","T":"2023-06-29T16:01:12.361Z","C":"internalcmd/used_space_filewalker.go:99","M":"used-space-filewalker completed","piecesTotal":1000,"piecesContentSize":488}`)
		require.NoError(t, err)

		require.Equal(t, 3, observedLogs.Len())
		logs := observedLogs.All()

		require.Contains(t, logs[0].Message, "Database started")
		require.Equal(t, logs[0].Caller.FullPath(), "internalcmd/used_space_filewalker.go:85")

		require.Contains(t, logs[1].Message, "used-space-filewalker started")
		require.Equal(t, logs[1].Caller.FullPath(), "internalcmd/used_space_filewalker.go:90")

		require.Contains(t, logs[2].Message, "used-space-filewalker completed")
		require.Equal(t, logs[2].Caller.FullPath(), "internalcmd/used_space_filewalker.go:99")
		require.Equal(t, float64(1000), logs[2].ContextMap()["piecesTotal"])
		require.Equal(t, float64(488), logs[2].ContextMap()["piecesContentSize"])
	})

	t.Run("invalid time key in iso8601 datetime with timezone", func(t *testing.T) {
		_, err := io.WriteString(w, `{"L":"INFO","T":"2023-06-25T18:21:16.181+0200","C":"internalcmd/used_space_filewalker.go:85","M":"Database started"}`)
		var expectedError *time.ParseError
		require.True(t, errors.As(err, &expectedError))
		require.Error(t, err)

		// https://github.com/storj/storj/issues/6006
		expectedErrMsg := `parsing time "2023-06-25T18:21:16.181+0200" as "2006-01-02T15:04:05Z07:00": cannot parse "+0200" as "Z07:00"`
		require.Contains(t, expectedErrMsg, err.Error())
	})
}
