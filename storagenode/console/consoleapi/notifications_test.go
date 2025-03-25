// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/notifications"
)

func TestNotificationsApi(t *testing.T) {
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount:   2,
			StorageNodeCount: 1,
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			satellite2 := planet.Satellites[1]
			sno := planet.StorageNodes[0]
			console := sno.Console
			notificationsDB := sno.DB.Notifications()
			baseURL := fmt.Sprintf("http://%s/api/notifications", console.Listener.Addr())

			newNotification1 := notifications.NewNotification{
				SenderID: satellite.ID(),
				Type:     0,
				Title:    "title1",
				Message:  "title1",
			}
			newNotification2 := notifications.NewNotification{
				SenderID: satellite2.ID(),
				Type:     0,
				Title:    "title2",
				Message:  "title2",
			}
			notif1, err := notificationsDB.Insert(ctx, newNotification1)
			require.NoError(t, err)
			require.Equal(t, newNotification1.Title, notif1.Title)
			require.Equal(t, newNotification1.Type, notif1.Type)
			require.Equal(t, newNotification1.SenderID, notif1.SenderID)

			notif2, err := notificationsDB.Insert(ctx, newNotification2)
			require.NoError(t, err)
			require.Equal(t, newNotification2.Title, notif2.Title)
			require.Equal(t, newNotification2.Type, notif2.Type)
			require.Equal(t, newNotification2.SenderID, notif2.SenderID)

			t.Run("ListNotifications", func(t *testing.T) {
				// should return notifications list.
				url := baseURL + "/list?limit=3&page=1"
				res, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				expected, err := json.Marshal([]notifications.Notification{notif2, notif1})
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := io.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, "{\"page\":{\"notifications\":"+string(expected)+",\"offset\":0,\"limit\":3,\"currentPage\":1,\"pageCount\":1},\"unreadCount\":2,\"totalCount\":2}"+"\n", string(body))
			})

			t.Run("ReadNotification", func(t *testing.T) {
				// should change status of notification by id to read.
				url := fmt.Sprintf("%s/%s/read", baseURL, notif1.ID.String())
				res, err := httpPost(ctx, url, "application/json", nil)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()

				notificationList, err := notificationsDB.List(ctx, notifications.Cursor{
					Limit: 2,
					Page:  1,
				})
				require.NoError(t, err)
				require.NotEqual(t, nil, notificationList.Notifications[1].ReadAt)
			})

			t.Run("ReadAllNotifications", func(t *testing.T) {
				// should change status of notification by id to read.
				url := baseURL + "/readall"
				res, err := httpPost(ctx, url, "application/json", nil)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()

				notificationList, err := notificationsDB.List(ctx, notifications.Cursor{
					Limit: 2,
					Page:  1,
				})
				require.NoError(t, err)
				require.NotEqual(t, nil, notificationList.Notifications[1].ReadAt)
				require.NotEqual(t, nil, notificationList.Notifications[0].ReadAt)
			})
		},
	)
}
