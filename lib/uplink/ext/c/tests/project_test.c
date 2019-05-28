// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include "unity.h"
#include "../../uplink-cgo.h"

void create_test_bucket(ProjectRef_t ref_project, char *bucket_name, Bucket_t *bucket, BucketConfig_t *cfg, char **err)
{
    *bucket = CreateBucket(ref_project, bucket_name, cfg, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    TEST_ASSERT_NOT_NULL(bucket.encryption_parameters);
    TEST_ASSERT_EQUAL(enc_param.cipher_suite, bucket.encryption_parameters->cipher_suite);
    TEST_ASSERT_EQUAL(enc_param.block_size, bucket.encryption_parameters->block_size);

    TEST_ASSERT_NOT_NULL(bucket.redundancy_scheme);
    TEST_ASSERT_EQUAL(scheme.algorithm, bucket.redundancy_scheme->algorithm);
    TEST_ASSERT_EQUAL(scheme.share_size, bucket.redundancy_scheme->share_size);
    TEST_ASSERT_EQUAL(scheme.required_shares, bucket.redundancy_scheme->required_shares);
    TEST_ASSERT_EQUAL(scheme.repair_shares, bucket.redundancy_scheme->repair_shares);
    TEST_ASSERT_EQUAL(scheme.optimal_shares, bucket.redundancy_scheme->optimal_shares);
    TEST_ASSERT_EQUAL(scheme.total_shares, bucket.redundancy_scheme->total_shares);

    TEST_ASSERT_EQUAL_STRING(bucket_name, bucket.name);
    TEST_ASSERT_NOT_EQUAL(0, bucket.created);
    // TODO: what is expected here (bucket.path_cipher is 1 when bucket_cfg.path_cipher is 0 and vice-versa)?
//    TEST_ASSERT_EQUAL(bucket_cfg.path_cipher, bucket.path_cipher);
    // TODO: what is expected here (bucket.segment_size is 67108864)?
//    TEST_ASSERT_EQUAL(1024, bucket.segment_size);
}

UplinkRef_t NewTestUplink(char **);

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

    ProjectRef_t ref_project = OpenTestProject(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_project);

    CloseProject(ref_project, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
}

void TestCreateBucket(void)
{
    char *_err = "";
    char **err = &_err;

    ProjectRef_t ref_project = OpenTestProject(err);
    TEST_ASSERT_EQUAL_STRING("", *err);

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

    char *bucket_name = getenv("CREATE_BUCKET_NAME");

    Bucket_t *bucket = malloc(sizeof(Bucket_t));
    create_test_bucket(ref_project, bucket_name, bucket, &bucket_cfg, err);
}

void TestDeleteBucket(void)
{
    char *_err = "";
    char **err = &_err;

    ProjectRef_t ref_project = OpenTestProject(err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    char *bucket_name = getenv("DELETE_BUCKET_NAME");

    DeleteBucket(ref_project, bucket_name, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
}

void TestListBuckets(void)
{
    char *_err = "";
    char **err = &_err;

    ProjectRef_t ref_project = OpenTestProject(err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    // TODO: test BucketListOptions_t
    BucketList_t bucket_list = ListBuckets(ref_project, NULL, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_FALSE(bucket_list.more);
    TEST_ASSERT_EQUAL(2, bucket_list.length);
    TEST_ASSERT_NOT_NULL(bucket_list.items);

    Bucket_t bucket;
    bucket = bucket_list.items[0];

    char *create_bucket_name = getenv("CREATE_BUCKET_NAME");
    TEST_ASSERT_EQUAL_STRING(create_bucket_name, bucket.name);
    TEST_ASSERT_NOT_EQUAL(0, bucket.created);

    bucket = bucket_list.items[1];

    char *delete_bucket_name = getenv("DELETE_BUCKET_NAME");
    TEST_ASSERT_EQUAL_STRING(delete_bucket_name, bucket.name);
    TEST_ASSERT_NOT_EQUAL(0, bucket.created);
}

void TestGetBucketInfo(void)
{
    char *_err = "";
    char **err = &_err;

    ProjectRef_t ref_project = OpenTestProject(err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    char *bucket_name = getenv("INFO_BUCKET_NAME");
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestCreateBucket);
    RUN_TEST(TestListBuckets);
    RUN_TEST(TestDeleteBucket);
    return UNITY_END();
}
