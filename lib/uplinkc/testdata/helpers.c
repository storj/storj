// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdio.h>
#include "unity.h"
#include "../../uplink-cgo.h"
#include <inttypes.h>

ProjectRef_t OpenTestProject(char **err)
{
    char *satellite_addr = getenv("SATELLITE_ADDR");
    APIKeyRef_t ref_apikey = parse_api_key(getenv("APIKEY"), err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    UplinkRef_t ref_uplink = NewUplinkInsecure(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_uplink);

    return open_project(ref_uplink, satellite_addr, ref_apikey, err);
}

Bucket_t *CreateTestBucket(ProjectRef_t ref_project, char *bucket_name, char **err)
{
    Bucket_t *bucket = malloc(sizeof(Bucket_t));

    EncryptionParameters_t enc_param;
    enc_param.cipher_suite = 1;
    enc_param.block_size = 1024;

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

    *bucket = create_bucket(ref_project, bucket_name, &bucket_cfg, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    TEST_ASSERT_EQUAL(enc_param.cipher_suite, bucket->encryption_parameters.cipher_suite);
    TEST_ASSERT_EQUAL(enc_param.block_size, bucket->encryption_parameters.block_size);

    TEST_ASSERT_EQUAL(scheme.algorithm, bucket->redundancy_scheme.algorithm);
    TEST_ASSERT_EQUAL(scheme.share_size, bucket->redundancy_scheme.share_size);
    TEST_ASSERT_EQUAL(scheme.required_shares, bucket->redundancy_scheme.required_shares);
    TEST_ASSERT_EQUAL(scheme.repair_shares, bucket->redundancy_scheme.repair_shares);
    TEST_ASSERT_EQUAL(scheme.optimal_shares, bucket->redundancy_scheme.optimal_shares);
    TEST_ASSERT_EQUAL(scheme.total_shares, bucket->redundancy_scheme.total_shares);

    TEST_ASSERT_EQUAL_STRING(bucket_name, bucket->name);
    TEST_ASSERT_NOT_EQUAL(0, bucket->created);
    // TODO: what is expected here (bucket.path_cipher is 1 when bucket_cfg.path_cipher is 0 and vice-versa)?
//    TEST_ASSERT_EQUAL(bucket_cfg.path_cipher, bucket.path_cipher);
    // TODO: what is expected here (bucket.segment_size is 67108864)?
    // TODO: reference same default constant
    TEST_ASSERT_EQUAL(67108864, bucket->segment_size);

    return bucket;
}

EncryptionAccess_t * NewEncryptionAccess(uint8_t *key, int key_len)
{
    EncryptionAccess_t *access = malloc(sizeof(EncryptionAccess_t));
    access->key = malloc(sizeof(Bytes_t));
    access->key->length = key_len;
    access->key->bytes = calloc(key_len, sizeof(uint8_t));

    memcpy(access->key->bytes, key, key_len);

    return access;
}

void FreeEncryptionAccess(EncryptionAccess_t *access)
{
    if (access != NULL) {
        if (access->key != NULL) {
            if (access->key->bytes != NULL) {
                free(access->key->bytes);
            }
            free(access->key);
        }
        free(access);
    }
}

void FreeBucket(Bucket_t *bucket)
{
    if (bucket != NULL) {
        free(bucket);
    }
}

Bytes_t *BytesFromString(char *str_data)
{
    size_t length = strlen(str_data);
    Bytes_t *data = malloc(length);
    data->bytes = (uint8_t *)str_data;
    data->length = strlen(str_data);
    return data;
}

void create_test_object(BucketRef_t ref_bucket, char *path, Object_t *object, Bytes_t *data, char **err)
{

	FILE *f = fmemopen(data->bytes, data->length, "r");

    UploadOptions_t opts = {
        "text/plain",
        0,
        time(NULL),
    };

    UploadObject(ref_bucket, path, f, &opts, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    fclose(f);
}