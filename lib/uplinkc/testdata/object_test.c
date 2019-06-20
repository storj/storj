// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdlib.h>

#include "require.h"
#include "uplink.h"
#include "helpers.h"

void handle_project(ProjectRef project);

int main(int argc, char *argv[]) {
    ProjectOptions opts = {};
    memset(&opts.key, '\0', 32);
    memcpy(&opts.key, "hello", 5);

    with_test_project(&handle_project, &opts);
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
    memset(&access.key, '\0', 32);
    memcpy(&access.key, "hello", 5);

    BucketRef bucket = open_bucket(project, bucket_name, access, err);
    require_noerror(*err);

    char *object_paths[] = {"TestObject1","TestObject2","TestObject3","TestObject4"};
    int num_of_objects = 4;

    // NB: about +500 years from time of writing
    int64_t future_expiration_timestamp = 17329017831;

    for(int i = 0; i < num_of_objects; i++) {
        int data_len = 1024 * pow((i + 1), 2);
        char *data = mkrndstr(data_len);

        { // upload
            MapRef map = new_map_ref();
            UploadOptions opts = {
                "text/plain",
                map,
                future_expiration_timestamp,
            };

            UploaderRef uploader = upload(bucket, object_paths[i], &opts, err);
            require_noerror(*err);

            delete_map_ref(map);

            int uploaded = 0;
            while (uploaded < data_len) {
                int write_len = upload_write(uploader, (uint8_t *)data+uploaded, 256, err);
                require_noerror(*err);

                if (write_len == 0) {
                    break;
                }

                uploaded += write_len;
            }

            upload_commit(uploader, err);
            require_noerror(*err);
        }

        { // download
            DownloaderRef downloader = download(bucket, object_paths[i], err);
            require_noerror(*err);

            char downloadedData[data_len];
            memset(downloadedData, '\0', data_len);
            int downloadedTotal = 0;

            uint8_t *bytes = NULL;

            while (downloadedTotal < data_len) {
                uint64_t size_to_read = 1024;
                bytes = malloc(size_to_read);
                uint64_t downloadedSize = download_read(downloader, bytes, size_to_read, err);

                if (downloadedSize == EOF) {
                    free(bytes);
                    break;
                }

                require_noerror(*err);
                memcpy(downloadedData+downloadedTotal, bytes, downloadedSize);
                downloadedTotal += downloadedSize;
                free(bytes);
            }

            download_close(downloader, err);
            require_noerror(*err);
            require(memcmp(data, downloadedData, data_len) == 0);
        }

        if (data != NULL) {
            free(data);
        }
    }

    close_bucket(bucket, err);
    require_noerror(*err);
}