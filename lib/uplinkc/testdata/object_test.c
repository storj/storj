// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include <time.h>

#include "uplink.h"
#include "helpers.h"

int main(int argc, char *argv[])
{
    char *_err = "";
    char **err = &_err;

    // Open Project
    ProjectRef_t ref_project = OpenTestProject(err);
    assert(strcmp("", *err) == 0);

    char *bucket_name = "TestBucket1";

    // Create buckets
    Bucket_t *bucket = CreateTestBucket(ref_project, bucket_name, err);
    assert(strcmp("", *err) == 0);
    free(bucket);

    uint8_t *enc_key = "abcdefghijklmnopqrstuvwxyzABCDEF";
    EncryptionAccess_t *access = NewEncryptionAccess(enc_key, strlen((const char *)enc_key));

    // Open bucket
    BucketRef_t ref_bucket = OpenBucket(ref_project, bucket_name, NULL, err);
    assert(strcmp("", *err) == 0);

    char *object_path = "TestObject1";

    // Create objects
    char *str_data = "testing data 123";
    Object_t *object = malloc(sizeof(Object_t));
    Bytes_t *data = BytesFromString(str_data);

    create_test_object(ref_bucket, object_path, object, data, err);
    assert(strcmp("", *err) == 0);
    free(object);

    ObjectRef_t object_ref = OpenObject(ref_bucket, object_path, err);
    assert(strcmp("", *err) == 0);

    ObjectMeta_t object_meta = ObjectMeta(object_ref, err);
    assert(strcmp("", *err) == 0);
    assert(strcmp(object_path, object_meta.Path) == 0);
    assert(data->length == object_meta.Size);

    DownloadReaderRef_t downloader = DownloadRange(object_ref, 0, object_meta.Size, err);
    assert(strcmp("", *err) == 0);

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
        assert(strcmp("", *err) == 0);
        memcpy(downloadedData+downloadedTotal, bytes->bytes, bytes->length);
        downloadedTotal += downloadedSize;
        free(bytes);
    }

    assert(strcmp(str_data, downloadedData) == 0);

    // Close Project
    CloseProject(ref_project, err);
    assert(strcmp("", *err) == 0);

    free(data);
}