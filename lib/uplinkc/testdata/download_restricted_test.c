// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdlib.h>

#include "require.h"

#include "uplink.h"
#include "helpers.h"

int main(int argc, char *argv[])
{
    char *_err = "";
    char **err = &_err;

    char *satellite_addr = getenv("SATELLITE_0_ADDR");
    char *apikeyStr = getenv("GATEWAY_0_API_KEY");
    char *tmp_dir = getenv("TMP_DIR");
    char *bucket_name = "test-bucket";
    char *file_name = "test-file";

    printf("using SATELLITE_0_ADDR: %s\n", satellite_addr);
    printf("using GATEWAY_0_API_KEY: %s\n", apikeyStr);

    // date to upload
    size_t data_len = 1024 * 50;
    uint8_t *data = malloc(data_len);
    fill_random_data(data, data_len);

    UplinkConfig cfg = {};
    cfg.Volatile.tls.skip_peer_ca_whitelist = true; // TODO: add CA Whitelist

    EncryptionAccessRef encryption_access;
    APIKeyRef apikey;

    {
        // New uplink
        UplinkRef uplink = new_uplink(cfg, tmp_dir, err);
        require_noerror(*err);
        requiref(uplink._handle != 0, "got empty uplink\n");

        {
            // parse api key
            apikey = parse_api_key(apikeyStr, err);
            require_noerror(*err);
            requiref(apikey._handle != 0, "got empty apikey\n");

            {
                // open a project
                ProjectRef project = open_project(uplink, satellite_addr, apikey, err);
                require_noerror(*err);
                requiref(project._handle != 0, "got empty project\n");

                uint8_t *salted_key = project_salted_key_from_passphrase(project,
                                                                         "It's dangerous to go alone, take this!",
                                                                         err);
                require_noerror(*err);

                encryption_access = new_encryption_access_with_default_key(salted_key);
                char *enc_ctx = serialize_encryption_access(encryption_access, err);
                require_noerror(*err);

                // create buckets
                BucketConfig config = test_bucket_config();
                BucketInfo info = create_bucket(project, bucket_name, &config, err);
                require_noerror(*err);
                free_bucket_info(&info);

                // open bucket
                BucketRef bucket = open_bucket(project, bucket_name, enc_ctx, err);
                require_noerror(*err);

                // NB: about +500 years from time of writing
                int64_t future_expiration_timestamp = 17329017831;

                UploadOptions opts = {
                    "text/plain",
                    future_expiration_timestamp,
                };

                // upload
                UploaderRef uploader = upload(bucket, file_name, &opts, err);
                require_noerror(*err);

                size_t uploaded_total = 0;
                while (uploaded_total < data_len)
                {
                    size_t size_to_write = (data_len - uploaded_total > 256) ? 256 : data_len - uploaded_total;

                    if (size_to_write == 0)
                    {
                        break;
                    }

                    size_t write_size = upload_write(uploader, (uint8_t *)data + uploaded_total, size_to_write, err);
                    require_noerror(*err);

                    if (write_size == 0)
                    {
                        break;
                    }

                    uploaded_total += write_size;
                }

                upload_commit(uploader, err);
                require_noerror(*err);

                free_uploader(uploader);

                close_bucket(bucket, err);
                require_noerror(*err);

                free(salted_key);

                // close project
                close_project(project, err);
                require_noerror(*err);
            }
        }

        // Close uplinks
        close_uplink(uplink, err);
        require_noerror(*err);
    }

    {
        // create new scope and restrict it

        ScopeRef scope = new_scope(satellite_addr, apikey, encryption_access, err);
        require_noerror(*err);

        Caveat caveat = {disallow_writes : true};
        EncryptionRestriction restrictions[] = {
            {
                bucket_name,
                file_name,
            },
        };
        ScopeRef restrictedScope = restrict_scope(scope, caveat, &restrictions[0], 1, err);
        require_noerror(*err);

        APIKeyRef restrictedApikey = get_scope_api_key(restrictedScope, err);
        require_noerror(*err);

        EncryptionAccessRef restrictedEncryptionAccess = get_scope_enc_access(restrictedScope, err);
        require_noerror(*err);

        {
            // New uplink
            UplinkRef uplink = new_uplink(cfg, tmp_dir, err);
            require_noerror(*err);
            requiref(uplink._handle != 0, "got empty uplink\n");

            {
                // open a project
                ProjectRef project = open_project(uplink, satellite_addr, restrictedApikey, err);
                require_noerror(*err);
                requiref(project._handle != 0, "got empty project\n");

                char *enc_ctx = serialize_encryption_access(restrictedEncryptionAccess, err);
                require_noerror(*err);

                // open bucket
                BucketRef bucket = open_bucket(project, bucket_name, enc_ctx, err);
                require_noerror(*err);

                // download
                DownloaderRef downloader = download(bucket, file_name, err);
                require_noerror(*err);

                size_t data_len = 1024 * 50;
                uint8_t *downloaded_data = malloc(data_len);
                memset(downloaded_data, '\0', data_len);
                size_t downloaded_total = 0;

                size_t size_to_read = 256;
                while (downloaded_total < data_len)
                {
                    size_t read_size = download_read(downloader, &downloaded_data[downloaded_total], size_to_read, err);
                    require_noerror(*err);

                    if (read_size == 0)
                    {
                        break;
                    }

                    downloaded_total += read_size;
                }

                download_close(downloader, err);
                require_noerror(*err);
                require(memcmp(data, downloaded_data, data_len) == 0);

                free(downloaded_data);
                free_downloader(downloader);

                close_bucket(bucket, err);
                require_noerror(*err);

                close_project(project, err);
                require_noerror(*err);
            }

            close_uplink(uplink, err);
            require_noerror(*err);
        }

        free_api_key(restrictedApikey);
        free_encryption_access(restrictedEncryptionAccess);

        free_scope(restrictedScope);
        free_scope(scope);
    }

    {
        // create new scope and restrict it

        ScopeRef scope = new_scope(satellite_addr, apikey, encryption_access, err);
        require_noerror(*err);

        Caveat caveat = {disallow_reads : true};
        EncryptionRestriction restrictions[] = {
            {
                bucket_name,
                "",
            },
        };
        ScopeRef restrictedScope = restrict_scope(scope, caveat, &restrictions[0], 1, err);
        require_noerror(*err);

        APIKeyRef restrictedApikey = get_scope_api_key(restrictedScope, err);
        require_noerror(*err);

        EncryptionAccessRef restrictedEncryptionAccess = get_scope_enc_access(restrictedScope, err);
        require_noerror(*err);

        {
            // New uplink
            UplinkRef uplink = new_uplink(cfg, tmp_dir, err);
            require_noerror(*err);
            requiref(uplink._handle != 0, "got empty uplink\n");

            {
                // open a project
                ProjectRef project = open_project(uplink, satellite_addr, restrictedApikey, err);
                require_noerror(*err);
                requiref(project._handle != 0, "got empty project\n");

                char *enc_ctx = serialize_encryption_access(restrictedEncryptionAccess, err);
                require_noerror(*err);

                // open bucket
                BucketRef bucket = open_bucket(project, bucket_name, enc_ctx, err);
                require_noerror(*err);

                // NB: about +500 years from time of writing
                int64_t future_expiration_timestamp = 17329017831;

                UploadOptions opts = {
                    "text/plain",
                    future_expiration_timestamp,
                };

                // upload
                UploaderRef uploader = upload(bucket, "new-test-file", &opts, err);
                require_noerror(*err);

                size_t uploaded_total = 0;
                while (uploaded_total < data_len)
                {
                    size_t size_to_write = (data_len - uploaded_total > 256) ? 256 : data_len - uploaded_total;

                    if (size_to_write == 0)
                    {
                        break;
                    }

                    size_t write_size = upload_write(uploader, (uint8_t *)data + uploaded_total, size_to_write, err);
                    require_noerror(*err);

                    if (write_size == 0)
                    {
                        break;
                    }

                    uploaded_total += write_size;
                }

                upload_commit(uploader, err);
                require_noerror(*err);

                free_uploader(uploader);

                close_bucket(bucket, err);
                require_noerror(*err);

                // close project
                close_project(project, err);
                require_noerror(*err);
            }

            close_uplink(uplink, err);
            require_noerror(*err);
        }

        free_api_key(apikey);
        free_encryption_access(encryption_access);

        free_api_key(restrictedApikey);
        free_encryption_access(restrictedEncryptionAccess);

        free_scope(restrictedScope);
        free_scope(scope);
    }

    free_api_key(apikey);
    free_encryption_access(encryption_access);

    requiref(internal_UniverseIsEmpty(), "universe is not empty\n");
}
