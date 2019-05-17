// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <signal.h>
#include <string.h>
#include "unity.h"
#include "../../uplink-cgo.h"

gvUplink *NewTestUplink(char **);

void TestCreateBucket(void)
{
    char *_err = "";
    char **err = &_err;
    char *satelliteAddr = getenv("SATELLITEADDR");
    gvAPIKey apiKey = ParseAPIKey(getenv("APIKEY"), err);
    uint8_t encryptionKey[32];

    struct ProjectOptions opts = {
        {&encryptionKey}};

    gvUplink *uplink = NewTestUplink(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_NULL(uplink);

    Project project = OpenProject(uplink->Ptr, satelliteAddr, apiKey.Ptr, opts, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
//
//    // TODO: replace with enum
//    struct BucketConfig cfg = {0};
//    struct Bucket bucket = CreateBucket(project, "testbucket", cfg, err);
//    TEST_ASSERT_EQUAL_STRING("", *err);
//    TEST_ASSERT_EQUAL_STRING("testbucket", bucket.Name);
}
