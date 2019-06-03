// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include <time.h>
#include "unity.h"
#include "../../uplink-cgo.h"

void TestObject(void)
{
    char *_err = "";
    char **err = &_err;

    // Open Project
    ProjectRef_t ref_project = OpenTestProject(err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    char *bucket_name = "TestBucket1";

    // Create buckets
    Bucket_t *bucket = CreateTestBucket(ref_project, bucket_name, err);
    free(bucket);

    uint8_t *enc_key = "abcdefghijklmnopqrstuvwxyzABCDEF";
    EncryptionAccess_t *access = NewEncryptionAccess(enc_key, strlen((const char *)enc_key));

    // Open bucket
    BucketRef_t opened_bucket = OpenBucket(ref_project, bucket_names[0], access, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    char *object_path = "TestObject1";

    // Create objects
    char *str_data = "testing data 123";
    Object_t *object = malloc(sizeof(Object_t));
    Bytes_t *data = BytesFromString(str_data);

    create_test_object(ref_bucket, object_path, object, data, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    free(object);

    // Close Project
    CloseProject(ref_project, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestObject);
    return UNITY_END();
}
