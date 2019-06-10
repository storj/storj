// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include <assert.h>

#include "require.h"
#include "uplink.h"


int main(int argc, char *argv[])
{
    char *_err = "";
    char **err = &_err;

    char *satellite_addr = getenv("SATELLITE_ADDR");
    char *apikey = getenv("APIKEY");

    APIKeyRef_t ref_apikey = ParseAPIKey(apikey, err);
    require_noerror(*err);

    // New insecure uplink
    UplinkRef_t uplink = NewUplinkInsecure(err);
    require_noerror(*err);
    require(0 != uplink, "got empty uplink\n");

    // OpenProject
    ProjectRef_t project = OpenProject(uplink, satellite_addr, ref_apikey, err);
    require_noerror(*err);
    require(0 != project, "got empty project\n");

    char *bucket_names[] = {"alpha", "beta", "gamma", "delta"};
    int num_of_buckets = sizeof(bucket_names) / sizeof(bucket_names[0]);

    for(size_t i = 0; i < num_of_buckets; i++) {
        EncryptionParameters_t enc_param = {};
        enc_param.cipher_suite = 1;
        enc_param.block_size = 1024;

        RedundancyScheme_t scheme = {};
        scheme.algorithm = 1;
        scheme.share_size = 1024;
        scheme.required_shares = 4;
        scheme.repair_shares = 6;
        scheme.optimal_shares = 8;
        scheme.total_shares = 10;

        BucketConfig_t bucket_cfg = {};
        bucket_cfg.path_cipher = 0;
        bucket_cfg.encryption_parameters = enc_param;
        bucket_cfg.redundancy_scheme = scheme;

        Bucket_t bucket = CreateBucket(project, bucket_names[i], &bucket_cfg, err);
        require_noerror(err);
        require(0 != bucket, "got empty bucket\n");

        free(bucket);
    }

    // Close uplink
    CloseUplink(uplink, err);
    require_noerror(*err);
}


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