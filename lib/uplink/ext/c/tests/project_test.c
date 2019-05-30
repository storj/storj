// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include "unity.h"
#include "../../uplink-cgo.h"
#include "helpers.h"

UplinkRef_t NewTestUplink(char **);

void create_test_bucket(ProjectRef_t ref_project, char *bucket_name, Bucket_t *bucket, char **err)
{
    EncryptionParameters_t enc_param;
    enc_param.cipher_suite = 1;
    enc_param.block_size = 1024;

    // NB: release defaults (maybe factor out into a lib helper)
    RedundancyScheme_t scheme;
    scheme.algorithm = 1;
    scheme.share_size = 1024;
    // TODO: we probably want to use dev defaults instead
    scheme.required_shares = 29;
    scheme.repair_shares = 35;
    scheme.optimal_shares = 80;
    scheme.total_shares = 95;

    BucketConfig_t bucket_cfg;
    bucket_cfg.path_cipher = 0;
    bucket_cfg.encryption_parameters = &enc_param;

    *bucket = CreateBucket(ref_project, bucket_name, bucket_cfg, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    TEST_ASSERT_NOT_NULL(bucket->encryption_parameters);
    TEST_ASSERT_EQUAL(enc_param.cipher_suite, bucket->encryption_parameters->cipher_suite);
    TEST_ASSERT_EQUAL(enc_param.block_size, bucket->encryption_parameters->block_size);

    TEST_ASSERT_NOT_NULL(bucket->redundancy_scheme);
    TEST_ASSERT_EQUAL(scheme.algorithm, bucket->redundancy_scheme->algorithm);
    TEST_ASSERT_EQUAL(scheme.share_size, bucket->redundancy_scheme->share_size);
    TEST_ASSERT_EQUAL(scheme.required_shares, bucket->redundancy_scheme->required_shares);
    TEST_ASSERT_EQUAL(scheme.repair_shares, bucket->redundancy_scheme->repair_shares);
    TEST_ASSERT_EQUAL(scheme.optimal_shares, bucket->redundancy_scheme->optimal_shares);
    TEST_ASSERT_EQUAL(scheme.total_shares, bucket->redundancy_scheme->total_shares);

    TEST_ASSERT_EQUAL_STRING(bucket_name, bucket->name);
    TEST_ASSERT_NOT_EQUAL(0, bucket->created);
    // TODO: what is expected here (bucket.path_cipher is 1 when bucket_cfg.path_cipher is 0 and vice-versa)?
//    TEST_ASSERT_EQUAL(bucket_cfg.path_cipher, bucket.path_cipher);
    // TODO: what is expected here (bucket.segment_size is 67108864)?
//    TEST_ASSERT_EQUAL(1024, bucket.segment_size);
}

void TestCloseProject(void)
{
    char *_err = "";
    char **err = &_err;

    //TODO: Fill in

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
        Bucket_t *bucket = malloc(sizeof(Bucket_t));
        create_test_bucket(ref_project, bucket_names[i], bucket, err);
        TEST_ASSERT_EQUAL_STRING("", *err);
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

        // TODO: add assertions for the rest of bucket_info's nested fields (here and in go)
        // in a way that doesn't involve a refactor that offends alex's delicate sensibilities.
    }

    // Open bucket
    // TODO: remove duplication
    uint8_t *enc_key = "bryanssecretkey";
    Bytes_t key;
    key.bytes = enc_key;
    key.length = strlen((const char *)enc_key);
    EncryptionAccess_t access;
    access.key = &key;

    BucketRef_t opened_bucket = OpenBucket(ref_project, bucket_names[0], &access, err);
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
