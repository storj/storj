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
    char *_err = "";
    char **err = &_err;

    char *bucket_names[] = {"TestBucket1", "TestBucket2", "TestBucket3", "TestBucket4"};
    int num_of_buckets = sizeof(bucket_names) / sizeof(bucket_names[0]);

    // Create buckets
    for (int i=0; i < num_of_buckets; i++) {
        char *bucket_name = bucket_names[i];

        BucketConfig config = {};

        config.path_cipher = 0;

        config.encryption_parameters.cipher_suite = 1; // TODO: make a named const
        config.encryption_parameters.block_size = 1024;

        config.redundancy_scheme.algorithm = 1; // TODO: make a named const
        config.redundancy_scheme.share_size = 1024;
        config.redundancy_scheme.required_shares = 4;
        config.redundancy_scheme.repair_shares = 6;
        config.redundancy_scheme.optimal_shares = 8;
        config.redundancy_scheme.total_shares = 10;

        BucketInfo info = CreateBucket(project, bucket_name, config, err);
        require_noerror(*err);

        require(strcmp(bucket_name, info.name) == 0, "name mismatch\n");
        require(info.created != 0, "created was 0");

        require(config.encryption_parameters.cipher_suite == info.encryption_parameters.cipher_suite, "cipher_suite mismatch\n");
        require(config.encryption_parameters.block_size   == info.encryption_parameters.block_size, "block_size mismatch\n");

        require(config.redundancy_scheme.algorithm        == info.redundancy_scheme.algorithm, "algorithm mismatch\n");
        require(config.redundancy_scheme.share_size       == info.redundancy_scheme.share_size, "share_size mismatch\n");
        require(config.redundancy_scheme.required_shares  == info.redundancy_scheme.required_shares, "required_shares mismatch\n");
        require(config.redundancy_scheme.repair_shares    == info.redundancy_scheme.repair_shares, "repair_shares mismatch\n");
        require(config.redundancy_scheme.optimal_shares   == info.redundancy_scheme.optimal_shares, "optimal_shares mismatch\n");
        require(config.redundancy_scheme.total_shares     == info.redundancy_scheme.total_shares, "total_shares mismatch\n");

        FreeBucketInfo(&bucketinfo)
    }

    // listing buckets
    BucketList bucket_list = ListBuckets(project, NULL, err);
    require_noerror(*err);
    require(bucket_list.more == FALSE);
    require(bucket_list.length == num_of_buckets);
    require(bucket_list.items != NULL);

    for(int i = 0; i < bucket_list.length; i++) {
        BucketInfo *info = &bucket_list.items[i];
        require(strcmp(info->name, bucket_names[0]) == 0);
        require(info->created != 0, "created was 0");
    }

    FreeBucketList(bucket_list);

    // getting bucket infos
    for(int i = 0; i < num_of_buckets; i++) {
        char *bucket_name = bucket_names[i];
        BucketInfo info = GetBucketInfo(project, bucket_name, err);
        require_noerror(err);
        require(strcmp(info->name, bucket_names[0]) == 0);
        require(info->created != 0, "created was 0");

        FreeBucketInfo(info);
    }
}

/*
void TestProject(void)
{
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
*/