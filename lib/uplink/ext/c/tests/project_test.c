// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include "unity.h"
#include "../../uplink-cgo.h"
#include "helpers.h"

UplinkRef_t NewTestUplink(char **);

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
    BucketRef_t opened_bucket = OpenBucket(ref_project, bucket_names[0], access, err);
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
