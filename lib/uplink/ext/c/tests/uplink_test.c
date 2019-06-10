// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include "unity.h"

void TestUplink(void)
{
    char *_err = "";
    char **err = &_err;
    char *satellite_addr = getenv("SATELLITE_ADDR");
    APIKeyRef_t ref_apikey = ParseAPIKey(getenv("APIKEY"), err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    // New uplink
    UplinkRef_t ref_uplink = NewUplink(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_uplink);

    // New insecure uplink (test network requires this)
    UplinkRef_t ref_test_uplink = NewUplinkInsecure(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_test_uplink);

    // OpenProject
    OpenProject(ref_test_uplink, satellite_addr, ref_apikey, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    // Close uplinks
    CloseUplink(ref_test_uplink, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    CloseUplink(ref_test_uplink, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestUplink);
    return UNITY_END();
}
