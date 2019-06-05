// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include <time.h>
#include "unity.h"
#include "../../uplink-cgo.h"
#include "helpers.h"

void callback(Bytes_t bytes, bool done) {
    printf("Hi\n");
}

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
    TEST_ASSERT_EQUAL_STRING("", *err);
    free(bucket);

    uint8_t *enc_key = "abcdefghijklmnopqrstuvwxyzABCDEF";
    EncryptionAccess_t *access = NewEncryptionAccess(enc_key, strlen((const char *)enc_key));

    // Open bucket
    BucketRef_t ref_bucket = OpenBucket(ref_project, bucket_name, NULL, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    char *object_path = "TestObject1";

    // Create objects
    char *str_data = "testing data 123";
    Object_t *object = malloc(sizeof(Object_t));
    Bytes_t *data = BytesFromString(str_data);

    create_test_object(ref_bucket, object_path, object, data, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    free(object);

    ObjectRef_t object_ref = OpenObject(ref_bucket, object_path, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    ObjectMeta_t object_meta = ObjectMeta(object_ref, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_EQUAL_STRING(object_path, object_meta.Path);

    DownloadReaderRef_t downloader = DownloadRange(object_ref, 0, object_meta.Size, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    char downloadedData[object_meta.Size];
    memset(downloadedData, '\0', object_meta.Size);
    int downloadedTotal = 0;

    while (true) {
        Bytes_t *bytes = malloc(sizeof(Bytes_t));
        uint64_t downloadedSize = Download(downloader, bytes, err);
        if (downloadedSize == EOF) {
            free(bytes);
            break;
        }
        TEST_ASSERT_EQUAL_STRING("", *err);
        memcpy(downloadedData+downloadedTotal, bytes->bytes, bytes->length);
        downloadedTotal += downloadedSize;
        free(bytes);
    }

    TEST_ASSERT_EQUAL_STRING(str_data, downloadedData);

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
