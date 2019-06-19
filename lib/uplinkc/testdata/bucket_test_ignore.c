// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include <time.h>
#include "unity.h"
#include "../../uplink-cgo.h"
#include "helpers.h"

void TestBucket(void)
{
    char *_err = "";
    char **err = &_err;
    char *bucket_name = "TestBucket";

    // Open Project
    ProjectRef_t ref_project = OpenTestProject(err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    EncryptionParameters_t enc_param;
    enc_param.cipher_suite = 1;
    enc_param.block_size = 4 * 1024;

    RedundancyScheme_t scheme;
    scheme.algorithm = 1;
    scheme.share_size = 1024;
    scheme.required_shares = 4;``
    scheme.repair_shares = 6;
    scheme.optimal_shares = 8;
    scheme.total_shares = 10;

    BucketConfig_t bucket_cfg;
    bucket_cfg.path_cipher = 0;
    bucket_cfg.encryption_parameters = enc_param;
    bucket_cfg.redundancy_scheme = scheme;

    create_bucket(ref_project, bucket_name, NULL, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    // TODO: Encryption access
    BucketRef_t ref_bucket = open_bucket(ref_project, bucket_name, NULL, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    char *object_paths[] = {"TestObject1","TestObject2","TestObject3","TestObject4"};
    int num_of_objects = 4;

    // Create objects
    char *str_data = "testing data 123";
    for (int i=0; i < num_of_objects; i++) {
        Object_t *object = malloc(sizeof(Object_t));
        Bytes_t *data = BytesFromString(str_data);

        create_test_object(ref_bucket, object_paths[i], object, data, err);
        TEST_ASSERT_EQUAL_STRING("", *err);
        free(object);
        free(data);
    }


}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestBucket);
    return UNITY_END();
}
