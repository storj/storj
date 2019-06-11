// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdlib.h>

#include "require.h"

#include "uplink.h"

int main(int argc, char *argv[])
{
    char *_err = "";
    char **err = &_err;

    char *satellite_addr = getenv("SATELLITE_0_ADDR");
    char *apikeyStr = getenv("GATEWAY_0_APIKEY");

    {
        UplinkConfig cfg = {};
        cfg.Volatile.TLS.SkipPeerCAWhitelist = 1; // TODO: add CA Whitelist

        // New uplink
        Uplink uplink = NewUplink(cfg, err);
        require_noerror(*err);
        require(uplink._handle != 0, "got empty uplink\n");

        {
            // parse api key
            APIKey apikey = ParseAPIKey(apikeyStr, err);
            require_noerror(*err);
            require(apikey._handle != 0, "got empty apikey\n");

            {
                // open a project
                Project project = OpenProject(uplink, satellite_addr, apikey, err);
                require_noerror(*err);
                require(project._handle != 0, "got empty project\n");

                HandleProject(project);

                // close project
                CloseProject(project, err);
                require_noerror(*err);
            }

            // free api key
            FreeAPIKey(apikey);
        }

        // Close uplinks
        CloseUplink(uplink, err);
        require_noerror(*err);
    }

    require(internal_UniverseIsEmpty(), "universe is not empty\n");
}

void HandleProject(Project project) {
    char *bucket_names[] = {"TestBucket1", "TestBucket2", "TestBucket3", "TestBucket4"};
    int num_of_buckets = sizeof(bucket_names) / sizeof(bucket_names[0]);

    // Create buckets
    for (int i=0; i < num_of_buckets; i++) {
        Bucket *bucket = CreateTestBucket(ref_project, bucket_names[i], err);
        free(bucket);
    }
}

void TestProject(void)
{
    char *_err = "";
    char **err = &_err;

    // Open Project
    ProjectRef_t ref_project = OpenTestProject(err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    char *bucket_names[] = {"TestBucket1","TestBucket2","TestBucket3","TestBucket4"};
    int num_of_buckets = sizeof(bucket_names) / sizeof(bucket_names[0]);

    // Create buckets
    for (int i=0; i < num_of_buckets; i++) {
        Bucket_t *bucket = CreateTestBucket(ref_project, bucket_names[i], err);
        free(bucket);
    }

    // List buckets
    // TODO: test BucketListOptions_t
    BucketList_t bucket_list = ListBuckets(ref_project, NULL, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_FALSE(bucket_list.more);
    TEST_ASSERT_EQUAL(num_of_buckets, bucket_list.length);
    TEST_ASSERT_NOT_NULL(bucket_list.items);

    for (int i=0; i < num_of_buckets; i++) {
        Bucket_t *bucket = &bucket_list.items[i];
        TEST_ASSERT_EQUAL_STRING(bucket_names[i], bucket->name);
        TEST_ASSERT_NOT_EQUAL(0, bucket->created);

        // Get bucket info
        BucketInfo_t bucket_info = GetBucketInfo(ref_project, bucket->name, err);
        TEST_ASSERT_EQUAL_STRING("", *err);
        TEST_ASSERT_EQUAL_STRING(bucket->name, bucket_info.bucket.name);
        TEST_ASSERT_NOT_EQUAL(0, bucket_info.bucket.created);
    }
    free(bucket_list.items);

    uint8_t *enc_key = "abcdefghijklmnopqrstuvwxyzABCDEF";
    EncryptionAccess_t *access = NewEncryptionAccess(enc_key, strlen((const char *)enc_key));

    // Open bucket
    BucketRef_t ref_open_bucket = OpenBucket(ref_project, bucket_names[0], access, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    // TODO: exercise functions that operate on an open bucket to add assertions

    // Delete Buckets
    for (int i=0; i < num_of_buckets; i++) {
        if (i%2 == 0) {
            DeleteBucket(ref_project, bucket_names[i], err);
            TEST_ASSERT_EQUAL_STRING("", *err);
        }
    }

    // Close Project
    CloseProject(ref_project, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestProject);
    return UNITY_END();
}
