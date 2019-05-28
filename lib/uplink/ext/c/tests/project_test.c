// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include "unity.h"
#include "../../uplink-cgo.h"

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

ProjectRef_t OpenTestProject(char **err)
{
    char *satellite_addr = getenv("SATELLITE_ADDR");
    APIKeyRef_t ref_apikey = ParseAPIKey(getenv("APIKEY"), err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    UplinkRef_t ref_uplink = NewUplinkInsecure(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_uplink);

    return OpenProject(ref_uplink, satellite_addr, ref_apikey, err);
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

    // Create Project
    ProjectRef_t ref_project = OpenTestProject(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    if (strcmp(*err, "") != 0) {
        goto end_project_test;
    }

    char *bucket_names[] = {"TestBucket1","bryansboringbucket"};
    int num_of_buckets = sizeof(bucket_names) / sizeof(bucket_names[0]);

    // Create buckets
    for (int i=0; i < num_of_buckets; i++) {
        Bucket_t *bucket = malloc(sizeof(Bucket_t));
        create_test_bucket(ref_project, bucket_names[i], bucket, err);
        TEST_ASSERT_EQUAL_STRING("", *err);
        if (strcmp(*err, "") != 0) {
            goto end_project_test;
        }
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
        Bucket_t *listed_bucket;
        listed_bucket = &bucket_list.items[i];
        TEST_ASSERT_EQUAL_STRING(bucket_names[i], listed_bucket->name);
        TEST_ASSERT_NOT_EQUAL(0, listed_bucket->created);
    }

    // Delete Buckets
    for (int i=0; i < num_of_buckets; i++) {
        DeleteBucket(ref_project, bucket_names[i], err);
        TEST_ASSERT_EQUAL_STRING("", *err);
        if (strcmp(*err, "") != 0) {
            goto end_project_test;
        }
    }

end_project_test:
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
