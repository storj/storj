// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdlib.h>

#include "require.h"
#include "uplink.h"
#include "helpers2.h"

void handle_project(ProjectRef project);

int main(int argc, char *argv[]) {
    with_test_project(&handle_project);
}

void handle_project(ProjectRef project) {
    char *_err = "";
    char **err = &_err;

    char *bucket_name = "TestBucket";

    BucketConfig config = test_bucket_config();
    BucketInfo info = create_bucket(project, bucket_name, &config, err);
    require_noerror(*err);
    free_bucket_info(&info);

    EncryptionAccess access = {};
    memcpy(&access.key[0], "hello", 5);
    BucketRef bucket = open_bucket(project, bucket_name, access, err);
    require_noerror(*err);
    {
        char *object_paths[] = {"TestObject1","TestObject2","TestObject3","TestObject4"};
        int num_of_objects = 4;

        for(int i = 0; i < num_of_objects; i++) {
            char *data = mkrndstr(1024*i^2);

            MapRef map = new_map_ref();
            UploadOptions opts = {
                "text/plain",
                map,
                time(NULL),
            };

            UploaderRef uploader = upload(bucket, object_paths[i], &opts, err);
            require_noerror(*err);

            int uploaded = 0;
            while (uploaded <= strlen(data)) {
                int write_len = upload_write(uploader, (uint8_t *)data+uploaded, 1024, err);
                require_noerror(*err);

                if (write_len == 0) {
                    break;
                }

                uploaded += write_len;
            }

            upload_close(uploader, err);
            require_noerror(*err);

            if (data != NULL) {
                free(data);
        }
     }

    }
    close_bucket(bucket, err);
    require_noerror(*err);
}

//void TestObject(void)
//{
//char *_err = "";
//char **err = &_err;
//
//// Open Project
//ProjectRef_t ref_project = OpenTestProject(err);
//TEST_ASSERT_EQUAL_STRING("", *err);
//
//char *bucket_name = "TestBucket1";
//
//// Create buckets
//Bucket_t *bucket = CreateTestBucket(ref_project, bucket_name, err);
//TEST_ASSERT_EQUAL_STRING("", *err);
//free(bucket);
//
//uint8_t *enc_key = "abcdefghijklmnopqrstuvwxyzABCDEF";
//EncryptionAccess_t *access = NewEncryptionAccess(enc_key, strlen((const char *)enc_key));
//
//// Open bucket
//BucketRef_t ref_bucket = OpenBucket(ref_project, bucket_name, NULL, err);
//TEST_ASSERT_EQUAL_STRING("", *err);
//
//char *object_path = "TestObject1";
//
//// Create objects
//char *str_data = "testing data 123";
//Object_t *object = malloc(sizeof(Object_t));
//Bytes_t *data = BytesFromString(str_data);
//
//create_test_object(ref_bucket, object_path, object, data, err);
//TEST_ASSERT_EQUAL_STRING("", *err);
//free(object);
//
//ObjectRef_t object_ref = OpenObject(ref_bucket, object_path, err);
//TEST_ASSERT_EQUAL_STRING("", *err);
//
//ObjectMeta_t object_meta = ObjectMeta(object_ref, err);
//TEST_ASSERT_EQUAL_STRING("", *err);
//TEST_ASSERT_EQUAL_STRING(object_path, object_meta.Path);
//TEST_ASSERT_EQUAL(data->length, object_meta.Size);
//
//DownloadReaderRef_t downloader = DownloadRange(object_ref, 0, object_meta.Size, err);
//TEST_ASSERT_EQUAL_STRING("", *err);
//
//char downloadedData[object_meta.Size];
//memset(downloadedData, '\0', object_meta.Size);
//int downloadedTotal = 0;
//
//while (true) {
//Bytes_t *bytes = malloc(sizeof(Bytes_t));
//uint64_t downloadedSize = Download(downloader, bytes, err);
//if (downloadedSize == EOF) {
//free(bytes);
//break;
//}
//TEST_ASSERT_EQUAL_STRING("", *err);
//memcpy(downloadedData+downloadedTotal, bytes->bytes, bytes->length);
//downloadedTotal += downloadedSize;
//free(bytes);
//}
//
//TEST_ASSERT_EQUAL_STRING(str_data, downloadedData);
//
//// Close Project
//CloseProject(ref_project, err);
//TEST_ASSERT_EQUAL_STRING("", *err);
//
//free(data);
//}