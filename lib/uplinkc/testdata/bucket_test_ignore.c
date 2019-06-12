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

    // TODO: test with different bucket configs
<<<<<<< HEAD:lib/uplinkc/testdata/bucket_test_ignore.c
    create_bucket(ref_project, bucket_name, NULL, err);
=======
    EncryptionParameters_t enc_param;
    enc_param.cipher_suite = 1;
    enc_param.block_size = 4 * 1024;

    RedundancyScheme_t scheme;
    scheme.algorithm = 1;
    scheme.share_size = 1024;
    scheme.required_shares = 4;
    scheme.repair_shares = 6;
    scheme.optimal_shares = 8;
    scheme.total_shares = 10;

    BucketConfig_t bucket_cfg;
    bucket_cfg.path_cipher = 0;
    bucket_cfg.encryption_parameters = enc_param;
    bucket_cfg.redundancy_scheme = scheme;

    CreateBucket(ref_project, bucket_name, &bucket_cfg, err);
>>>>>>> cbindings:lib/uplink/ext/c/tests/bucket_test.c
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

    // List objects
    // TODO: test list options
    ObjectList_t objects_list = ListObjects(ref_bucket, NULL, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_EQUAL_STRING(bucket_name, objects_list.bucket);
    TEST_ASSERT_EQUAL_STRING("", objects_list.prefix);
    TEST_ASSERT_EQUAL(false, objects_list.more);
    TEST_ASSERT_EQUAL(num_of_objects, objects_list.length);

    Object_t *object;
    for (int i=0; i < objects_list.length; i++) {
        object = &objects_list.items[i];
        TEST_ASSERT_EQUAL_STRING(object_paths[i], object->path);
    }
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestBucket);
    return UNITY_END();
}
