// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include <assert.h>

#include "uplink.h"
#include "helpers.h"

int main(int argc, char *argv[])
{
    char *_err = "";
    char **err = &_err;

    // Open Project
    ProjectRef_t ref_project = OpenTestProject(err);
    assert(strcmp("", *err) == 0);

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
    assert(strcmp("", *err) == 0);
    assert(!bucket_list.more);
    assert(num_of_buckets == bucket_list.length);
    assert(bucket_list.items != NULL);

    for (int i=0; i < num_of_buckets; i++) {
        Bucket_t *bucket = &bucket_list.items[i];
        assert(strcmp(bucket_names[i], bucket->name) == 0);
        assert(0 != bucket->created);

        // Get bucket info
        BucketInfo_t bucket_info = GetBucketInfo(ref_project, bucket->name, err);
        assert(strcmp("", *err) == 0);
        assert(strcmp(bucket->name, bucket_info.bucket.name) == 0);
        assert(0 != bucket_info.bucket.created);
    }
    free(bucket_list.items);

    uint8_t *enc_key = "abcdefghijklmnopqrstuvwxyzABCDEF";
    EncryptionAccess_t *access = NewEncryptionAccess(enc_key, strlen((const char *)enc_key));

    // Open bucket
    BucketRef_t ref_open_bucket = OpenBucket(ref_project, bucket_names[0], access, err);
    assert(strcmp("", *err) == 0);

    // TODO: exercise functions that operate on an open bucket to add assertions

    // Delete Buckets
    for (int i=0; i < num_of_buckets; i++) {
        if (i%2 == 0) {
            DeleteBucket(ref_project, bucket_names[i], err);
            assert(strcmp("", *err) == 0);
        }
    }

    // Close Project
    CloseProject(ref_project, err);
    assert(strcmp("", *err) == 0);
}