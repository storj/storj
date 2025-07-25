// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewasm_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console/consolewasm"
	"storj.io/uplink"
)

func TestSetPermissionWithBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		satelliteNodeURL := satellitePeer.NodeURL().String()
		uplinkPeer := planet.Uplinks[0]
		APIKey := uplinkPeer.APIKey[satellitePeer.ID()]
		apiKeyString := APIKey.Serialize()
		projectID := uplinkPeer.Projects[0].ID
		require.Equal(t, 1, len(uplinkPeer.Projects))
		passphrase := "supersecretpassphrase"

		// Create an access grant with the uplink API key. With that access grant, create 2 buckets and upload an object.
		uplinkAccess, err := uplinkPeer.Config.RequestAccessWithPassphrase(ctx, satelliteNodeURL, apiKeyString, passphrase)
		require.NoError(t, err)
		uplinkPeer.Access[satellitePeer.ID()] = uplinkAccess
		testbucket1 := "buckettest1"
		testbucket2 := "buckettest2"
		testfilename := "file.txt"
		testdata := []byte("fun data")
		require.NoError(t, uplinkPeer.TestingCreateBucket(ctx, satellitePeer, testbucket1))
		require.NoError(t, uplinkPeer.TestingCreateBucket(ctx, satellitePeer, testbucket2))
		require.NoError(t, uplinkPeer.Upload(ctx, satellitePeer, testbucket1, testfilename, testdata))
		require.NoError(t, uplinkPeer.Upload(ctx, satellitePeer, testbucket2, testfilename, testdata))
		data, err := uplinkPeer.Download(ctx, satellitePeer, testbucket1, testfilename)
		require.NoError(t, err)
		require.Equal(t, data, testdata)

		buckets := []string{testbucket1}

		// Restrict the uplink access grant with read only permissions and only allows actions for 1 bucket.
		var sharePrefixes []uplink.SharePrefix
		for _, path := range buckets {
			sharePrefixes = append(sharePrefixes, uplink.SharePrefix{
				Bucket: path,
			})
		}
		restrictedUplinkAccess, err := uplinkAccess.Share(uplink.ReadOnlyPermission(), sharePrefixes...)
		require.NoError(t, err)

		// Expect that we can download the object with the restricted access for the 1 allowed bucket.
		uplinkPeer.Access[satellitePeer.ID()] = restrictedUplinkAccess
		uplinkPeer.APIKey[satellitePeer.ID()] = APIKey
		data, err = uplinkPeer.Download(ctx, satellitePeer, testbucket1, testfilename)
		require.NoError(t, err)
		require.Equal(t, data, testdata)
		err = uplinkPeer.Upload(ctx, satellitePeer, testbucket1, "file2", testdata)
		require.True(t, errors.Is(err, uplink.ErrPermissionDenied))
		_, err = uplinkPeer.Download(ctx, satellitePeer, testbucket2, testfilename)
		require.Error(t, err)

		// Create restricted access with the console access grant code that allows full access to only 1 bucket.
		readOnlyPermission := consolewasm.Permission{
			AllowDownload: true,
			AllowUpload:   false,
			AllowList:     true,
			AllowDelete:   false,
			NotBefore:     time.Now().Add(-24 * time.Hour),
			NotAfter:      time.Now().Add(48 * time.Hour),
		}
		restrictedKey, err := consolewasm.SetPermission(apiKeyString, buckets, readOnlyPermission)
		require.NoError(t, err)

		client := newTestClient(t, ctx, planet)
		user := client.defaultUser()
		client.login(user.email, user.password)

		resp, bodyString := client.request(http.MethodGet, fmt.Sprintf("/projects/%s/salt", projectID.String()), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var b64Salt string
		require.NoError(t, json.Unmarshal([]byte(bodyString), &b64Salt))

		restrictedAccessGrant, err := consolewasm.GenAccessGrant(satelliteNodeURL, restrictedKey.Serialize(), passphrase, b64Salt, true)
		require.NoError(t, err)
		restrictedAccess, err := uplink.ParseAccess(restrictedAccessGrant)
		require.NoError(t, err)

		// Expect that we can download the object with the restricted access for the 1 allowed bucket.
		uplinkPeer.APIKey[satellitePeer.ID()] = restrictedKey
		uplinkPeer.Access[satellitePeer.ID()] = restrictedAccess
		data, err = uplinkPeer.Download(ctx, satellitePeer, testbucket1, testfilename)
		require.NoError(t, err)
		require.Equal(t, data, testdata)
		err = uplinkPeer.Upload(ctx, satellitePeer, testbucket1, "file2", testdata)
		require.True(t, errors.Is(err, uplink.ErrPermissionDenied))
		_, err = uplinkPeer.Download(ctx, satellitePeer, testbucket2, testfilename)
		require.Error(t, err)
	})
}

func TestSetPermission_Uplink(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		satelliteNodeURL := satellitePeer.NodeURL().String()
		uplinkPeer := planet.Uplinks[0]
		APIKey := uplinkPeer.APIKey[satellitePeer.ID()]
		apiKeyString := APIKey.Serialize()
		require.Equal(t, 1, len(uplinkPeer.Projects))
		passphrase := "supersecretpassphrase"

		// Create an access grant with the uplink API key. With that access grant, create 2 bucket and upload files to them
		uplinkAccess, err := uplinkPeer.Config.RequestAccessWithPassphrase(ctx, satelliteNodeURL, apiKeyString, passphrase)
		require.NoError(t, err)
		uplinkPeer.Access[satellitePeer.ID()] = uplinkAccess
		testbucket1 := "buckettest1"
		testbucket2 := "buckettest2"
		testbucket3 := "buckettest3"
		testfilename1 := "file1.txt"
		testfilename2 := "file2.txt"
		testdata := []byte("fun data")
		require.NoError(t, uplinkPeer.TestingCreateBucket(ctx, satellitePeer, testbucket1))
		require.NoError(t, uplinkPeer.TestingCreateBucket(ctx, satellitePeer, testbucket2))
		require.NoError(t, uplinkPeer.Upload(ctx, satellitePeer, testbucket1, testfilename1, testdata))
		require.NoError(t, uplinkPeer.Upload(ctx, satellitePeer, testbucket2, testfilename1, testdata))
		require.NoError(t, uplinkPeer.Upload(ctx, satellitePeer, testbucket2, testfilename2, testdata))

		client := newTestClient(t, ctx, planet)
		user := client.defaultUser()
		client.login(user.email, user.password)

		resp, bodyString := client.request(http.MethodGet, fmt.Sprintf("/projects/%s/salt", uplinkPeer.Projects[0].ID.String()), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var b64Salt string
		require.NoError(t, json.Unmarshal([]byte(bodyString), &b64Salt))

		withAccessKey(ctx, t, planet, passphrase, b64Salt, "only delete", []string{}, consolewasm.Permission{AllowDelete: true}, func(t *testing.T, project *uplink.Project) {
			// All operation except delete should be restricted
			_, err = project.CreateBucket(ctx, testbucket2)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			upload, err := project.UploadObject(ctx, testbucket2, testfilename2, nil)
			require.NoError(t, err)
			_, err = upload.Write(testdata)
			require.NoError(t, err)
			err = upload.Commit()
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			_, err = project.DownloadObject(ctx, testbucket2, testfilename1, nil)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			// We can see buckets names, but not the files inside
			buckets := getAllBuckets(ctx, project)
			require.Equal(t, 2, len(buckets))

			objects := getAllObjects(ctx, project, testbucket1)
			require.Equal(t, 0, len(objects))

			_, err = project.DeleteObject(ctx, testbucket1, testfilename2)
			require.NoError(t, err)

			// Current implementation needs also permission to Download/Read/List so having
			// only Delete permission for DeleteBucketWithObjects won't work
			_, err = project.DeleteBucketWithObjects(ctx, testbucket2)
			require.Error(t, err)
		})

		withAccessKey(ctx, t, planet, passphrase, b64Salt, "only list", []string{testbucket1}, consolewasm.Permission{AllowList: true}, func(t *testing.T, project *uplink.Project) {
			// All operation except list inside testbucket1 should be restricted
			_, err = project.CreateBucket(ctx, testbucket2)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			upload, err := project.UploadObject(ctx, testbucket1, testfilename2, nil)
			require.NoError(t, err)
			_, err = upload.Write(testdata)
			require.NoError(t, err)
			err = upload.Commit()
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			_, err = project.DownloadObject(ctx, testbucket1, testfilename1, nil)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			// Only one bucket should be visible
			buckets := getAllBuckets(ctx, project)
			require.Equal(t, 1, len(buckets))

			objects := getAllObjects(ctx, project, testbucket1)
			require.Equal(t, 1, len(objects))

			_, err = project.DeleteObject(ctx, testbucket1, testfilename1)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			_, err = project.DeleteBucketWithObjects(ctx, testbucket1)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))
		})

		withAccessKey(ctx, t, planet, passphrase, b64Salt, "only upload", []string{testbucket1}, consolewasm.Permission{AllowUpload: true}, func(t *testing.T, project *uplink.Project) {
			// All operation except upload to the testbucket1 should be restricted
			_, err = project.CreateBucket(ctx, testbucket2)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			upload, err := project.UploadObject(ctx, testbucket1, testfilename2, nil)
			require.NoError(t, err)
			_, err = upload.Write(testdata)
			require.NoError(t, err)
			err = upload.Commit()
			require.NoError(t, err)

			_, err = project.DownloadObject(ctx, testbucket1, testfilename1, nil)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			// Only one bucket should be visible
			buckets := getAllBuckets(ctx, project)
			require.Equal(t, 1, len(buckets))

			objects := getAllObjects(ctx, project, testbucket1)
			require.Equal(t, 0, len(objects))

			_, err = project.DeleteObject(ctx, testbucket1, testfilename1)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			_, err = project.DeleteBucketWithObjects(ctx, testbucket1)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))
		})

		withAccessKey(ctx, t, planet, passphrase, b64Salt, "only download", []string{testbucket1}, consolewasm.Permission{AllowDownload: true}, func(t *testing.T, project *uplink.Project) {
			// All operation except download from testbucket1 should be restricted
			_, err = project.CreateBucket(ctx, testbucket2)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			upload, err := project.UploadObject(ctx, testbucket1, testfilename2, nil)
			require.NoError(t, err)

			_, err = upload.Write(testdata)
			require.NoError(t, err)
			err = upload.Commit()
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			download, err := project.DownloadObject(ctx, testbucket1, testfilename1, nil)
			require.NoError(t, err)
			require.NoError(t, download.Close())

			// Only one bucket should be visible
			buckets := getAllBuckets(ctx, project)
			require.Equal(t, 1, len(buckets))

			objects := getAllObjects(ctx, project, testbucket1)
			require.Equal(t, 0, len(objects))

			_, err = project.DeleteObject(ctx, testbucket1, testfilename1)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			_, err = project.DeleteBucketWithObjects(ctx, testbucket1)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))
		})

		withAccessKey(ctx, t, planet, passphrase, b64Salt, "not after", []string{}, consolewasm.Permission{
			AllowDownload: true,
			AllowUpload:   true,
			AllowList:     true,
			AllowDelete:   true,
			NotAfter:      time.Now().Add(-2 * time.Hour),
		}, func(t *testing.T, project *uplink.Project) {
			// All operation should be restricted
			_, err = project.CreateBucket(ctx, testbucket2)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			upload, err := project.UploadObject(ctx, testbucket1, testfilename2, nil)
			require.NoError(t, err)
			_, err = upload.Write(testdata)
			require.NoError(t, err)
			err = upload.Commit()
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			_, err = project.DownloadObject(ctx, testbucket1, testfilename1, nil)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			_, err = project.DeleteBucketWithObjects(ctx, testbucket2)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			buckets := getAllBuckets(ctx, project)
			require.Equal(t, 0, len(buckets))

			objects := getAllObjects(ctx, project, testbucket1)
			require.Equal(t, 0, len(objects))

			_, err = project.DeleteObject(ctx, testbucket1, testfilename1)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))
		})

		withAccessKey(ctx, t, planet, passphrase, b64Salt, "not before", []string{}, consolewasm.Permission{
			AllowDownload: true,
			AllowUpload:   true,
			AllowList:     true,
			AllowDelete:   true,
			NotBefore:     time.Now().Add(2 * time.Hour),
		}, func(t *testing.T, project *uplink.Project) {
			// All operation should be restricted
			_, err = project.CreateBucket(ctx, testbucket2)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			upload, err := project.UploadObject(ctx, testbucket2, testfilename2, nil)
			require.NoError(t, err)
			_, err = upload.Write(testdata)
			require.NoError(t, err)
			err = upload.Commit()
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			_, err = project.DownloadObject(ctx, testbucket1, testfilename1, nil)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			_, err = project.DeleteBucketWithObjects(ctx, testbucket2)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))

			buckets := getAllBuckets(ctx, project)
			require.Equal(t, 0, len(buckets))

			objects := getAllObjects(ctx, project, testbucket1)
			require.Equal(t, 0, len(objects))

			_, err = project.DeleteObject(ctx, testbucket1, testfilename1)
			require.True(t, errors.Is(err, uplink.ErrPermissionDenied))
		})

		oneHour := time.Hour
		withAccessKey(ctx, t, planet, passphrase, b64Salt, "max object ttl", []string{}, consolewasm.Permission{
			AllowDownload: true,
			AllowUpload:   true,
			AllowList:     true,
			AllowDelete:   true,
			MaxObjectTTL:  &oneHour,
		}, func(t *testing.T, project *uplink.Project) {
			_, err = project.EnsureBucket(ctx, testbucket3)
			require.NoError(t, err)

			upload, err := project.UploadObject(ctx, testbucket3, testfilename2, nil)
			require.NoError(t, err)
			_, err = upload.Write(testdata)
			require.NoError(t, err)
			err = upload.Commit()
			require.NoError(t, err)

			object, err := project.StatObject(ctx, testbucket3, testfilename2)
			require.NoError(t, err)
			require.WithinDuration(t, time.Now().Add(oneHour), object.System.Expires, time.Minute)

			_, err = project.DeleteBucketWithObjects(ctx, testbucket3)
			require.NoError(t, err)
		})

		withAccessKey(ctx, t, planet, passphrase, b64Salt, "all", []string{}, consolewasm.Permission{
			AllowDownload: true,
			AllowUpload:   true,
			AllowList:     true,
			AllowDelete:   true,
			NotBefore:     time.Now().Add(-24 * time.Hour),
			NotAfter:      time.Now().Add(24 * time.Hour),
			MaxObjectTTL:  &oneHour,
		}, func(t *testing.T, project *uplink.Project) {
			// All operation allowed
			_, err = project.CreateBucket(ctx, testbucket3)
			require.NoError(t, err)

			upload, err := project.UploadObject(ctx, testbucket3, testfilename2, nil)
			require.NoError(t, err)
			_, err = upload.Write(testdata)
			require.NoError(t, err)
			err = upload.Commit()
			require.NoError(t, err)

			objects := getAllObjects(ctx, project, testbucket3)
			require.Equal(t, 1, len(objects))
			require.WithinDuration(t, time.Now().Add(oneHour), objects[0].System.Expires, time.Minute)

			download, err := project.DownloadObject(ctx, testbucket3, testfilename2, nil)
			require.NoError(t, err)
			require.NoError(t, download.Close())

			_, err = project.DeleteBucketWithObjects(ctx, testbucket3)
			require.NoError(t, err)

			buckets := getAllBuckets(ctx, project)
			require.Equal(t, 2, len(buckets))

			_, err = project.DeleteObject(ctx, testbucket1, testfilename1)
			require.NoError(t, err)
		})

	})
}

func getAllObjects(ctx *testcontext.Context, project *uplink.Project, bucket string) []*uplink.Object {
	var objects = []*uplink.Object{}
	iter := project.ListObjects(ctx, bucket, &uplink.ListObjectsOptions{System: true})
	for iter.Next() {
		objects = append(objects, iter.Item())
	}
	return objects
}

func getAllBuckets(ctx *testcontext.Context, project *uplink.Project) []*uplink.Bucket {
	var buckets = []*uplink.Bucket{}
	iter := project.ListBuckets(ctx, nil)
	for iter.Next() {
		buckets = append(buckets, iter.Item())
	}
	return buckets
}

func withAccessKey(ctx *testcontext.Context, t *testing.T, planet *testplanet.Planet, passphrase, salt, testname string, bucket []string, permissions consolewasm.Permission, fn func(t *testing.T, uplink *uplink.Project)) {
	t.Run(testname, func(t *testing.T) {
		upl := planet.Uplinks[0]
		sat := planet.Satellites[0]

		apikey := upl.APIKey[sat.ID()]
		restrictedKey, err := consolewasm.SetPermission(apikey.Serialize(), bucket, permissions)
		require.NoError(t, err)

		restrictedGrant, err := consolewasm.GenAccessGrant(sat.NodeURL().String(), restrictedKey.Serialize(), passphrase, salt, true)
		require.NoError(t, err)

		access, err := uplink.ParseAccess(restrictedGrant)
		require.NoError(t, err)

		project, err := uplink.OpenProject(ctx, access)
		require.NoError(t, err)

		defer func() { require.NoError(t, project.Close()) }()
		fn(t, project)
	})
}
