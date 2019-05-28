// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include "unity.h"
#include "../../uplink-cgo.h"

// TODO: remove duplication of this function
ProjectRef_t OpenTestProject(char **err)
{
    char *satellite_addr = getenv("SATELLITE_ADDR");
    APIKeyRef_t ref_apikey = ParseAPIKey(getenv("APIKEY"), err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    UplinkRef_t ref_uplink = NewUplinkInsecure(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_uplink);

    return OpenProject(ref_uplink, satellite_addr, ref_apikey, err);
}

void TestBucket(void)
{
    char *_err = "";
    char **err = &_err;
    char *bucket_name = getenv("BUCKET_NAME");

    // Open Project
    ProjectRef_t ref_project = OpenTestProject(err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    // TODO/WIP: open bucket and upload
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestBucket);
    return UNITY_END();
}
