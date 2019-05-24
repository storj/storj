// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <signal.h>
#include <string.h>
#include "unity.h"
#include "../../uplink-cgo.h"

UplinkRef_t NewTestUplink(char **);

void TestCreateBucket(void)
{
    char *_err = "";
    char **err = &_err;
    char *satellite_addr = getenv("SATELLITEADDR");
    APIKeyRef_t ref_apikey = ParseAPIKey(getenv("APIKEY"), err);

    UplinkRef_t ref_uplink = NewUplink(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_uplink);

    ProjectRef_t ref_project = OpenProject(ref_uplink, satellite_addr, ref_apikey, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    EncryptionParameters_t enc_param = STORJ__LIBUPLINK__ENCRYPTION_PARAMETERS__INIT;
    enc_param.cipher_suite = 0;
    enc_param.block_size = 1024;

    // NB: dev defaults (maybe factor out into a lib helper)
    RedundancyScheme_t scheme = STORJ__LIBUPLINK__REDUNDANCY_SCHEME__INIT;
    scheme.algorithm = 1;
    scheme.share_size = 1024;
    scheme.required_shares = 4;
    scheme.repair_shares = 6;
    scheme.optimal_shares = 8;
    scheme.total_shares = 10;

    BucketConfig_t bucket_cfg = STORJ__LIBUPLINK__BUCKET_CONFIG__INIT;
    bucket_cfg.path_cipher = 0;
    bucket_cfg.encryption_parameters = &enc_param;

    char *bucket_name = "testbucket";

    Bucket_t bucket = CreateBucket(ref_project, bucket_name, bucket_cfg, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    TEST_ASSERT_EQUAL_STRING(bucket_name, bucket.name);
}
