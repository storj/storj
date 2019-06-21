// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdlib.h>

#include "require.h"
#include "uplink.h"
#include "helpers.h"

void handle_project(ProjectRef project);

int main(int argc, char *argv[]) {
    ProjectOptions opts = {{0}};
    memcpy(&opts.key, "hello", 5);

    with_test_project(&handle_project, &opts);
}

void handle_project(ProjectRef project) {
    char *_err = "";
    char **err = &_err;

    char *bucket_name = "test-bucket";
    EncryptionAccess access = {{0}};
    memcpy(&access.key, "hello", 5);

    char *object_paths[] = {"test-object1","test-object2","test-object3","test-object4"};
    int num_of_objects = 4;

    // NB: about +500 years from time of writing
    int64_t future_expiration_timestamp = 17329017831;

    { // create buckets
        BucketConfig config = test_bucket_config();
        BucketInfo info = create_bucket(project, bucket_name, &config, err);
        require_noerror(*err);
        free_bucket_info(&info);
    }

    // open bucket
    BucketRef bucket = open_bucket(project, bucket_name, access, err);
    require_noerror(*err);


    for(int i = 0; i < num_of_objects; i++) {
        size_t data_len = 1024 * (i + 1) * (i + 1);
        uint8_t *data = malloc(data_len);
        fill_random_data(data, data_len);

        { // upload
            UploadOptions opts = {
                "text/plain",
                future_expiration_timestamp,
            };

            UploaderRef uploader = upload(bucket, object_paths[i], &opts, err);
            require_noerror(*err);

            size_t uploaded = 0;
            while (uploaded < data_len) {
                size_t to_write_len = (data_len - uploaded > 256) ? 256 : data_len - uploaded;
                size_t write_len = upload_write(uploader, (uint8_t *)data+uploaded, to_write_len, err);
                require_noerror(*err);

                if (write_len == 0) {
                    break;
                }

                uploaded += write_len;
            }

            upload_commit(uploader, err);
            require_noerror(*err);
        }

        { // object meta
            ObjectRef object_ref = open_object(bucket, object_paths[i], err);
            require_noerror(*err);

            ObjectMeta object_meta = get_object_meta(object_ref, err);
            require_noerror(*err);
            require(strcmp(object_paths[i], object_meta.path) == 0);
            require(data_len == object_meta.size);
            require(future_expiration_timestamp == object_meta.expires);
            require((time(NULL) - object_meta.created) <= 2);
            require((time(NULL) - object_meta.modified) <= 2);
            // TODO: checksum is empty, is this expected?
//            require(object_meta.checksum_bytes != NULL);
//            require(object_meta.checksum_length != 0);

            free_object_meta(&object_meta);
            close_object(object_ref, err);
        }

        { // download
            DownloaderRef downloader = download(bucket, object_paths[i], err);
            require_noerror(*err);

            uint8_t downloadedData[data_len];
            memset(downloadedData, '\0', data_len);
            size_t downloadedTotal = 0;

            size_t size_to_read = 256 + i;
            while (true) {
                size_t downloadedSize = download_read(downloader, &downloadedData[downloadedTotal], size_to_read, err);
                require_noerror(*err);

                if (downloadedSize == 0) {
                    break;
                }

                downloadedTotal += downloadedSize;
            }

            download_close(downloader, err);
            require_noerror(*err);
            require(memcmp(data, downloadedData, data_len) == 0);
        }

        if (data != NULL) {
            free(data);
        }

        require_noerror(*err);
    }

    { // List objects
        ObjectList objects_list = list_objects(bucket, NULL, err);
        require_noerror(*err);
        require(strcmp(bucket_name, objects_list.bucket) == 0);
        require(strcmp("", objects_list.prefix) == 0);
        require(false == objects_list.more);
        require(num_of_objects == objects_list.length);

        ObjectInfo *object;
        for (int i=0; i < objects_list.length; i++) {
            object = &objects_list.items[i];
            require(true == array_contains(object->path, object_paths, num_of_objects));
        }

        free_list_objects(&objects_list);
    }

    close_bucket(bucket, err);
    require_noerror(*err);
}