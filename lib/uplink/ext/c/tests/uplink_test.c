// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <signal.h>
#include <string.h>
#include "unity.h"
#include "../../uplink-cgo.h"

void TestNewUplink(void)
{
    char *_err = "";
    char **err = &_err;

    UplinkRef_t ref_uplink = NewUplink(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_uplink);
}


void TestCloseUplink(void)
{
    char *_err = "";
    char **err = &_err;

    UplinkRef_t ref_uplink = NewUplink(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_uplink);

    CloseUplink(ref_uplink, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
}

void TestOpenProject(void)
{
    char *_err = "";
    char **err = &_err;
    char *satellite_addr = getenv("SATELLITE_ADDR");
    APIKeyRef_t ref_apikey = ParseAPIKey(getenv("APIKEY"), err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    UplinkRef_t ref_uplink = NewUplinkInsecure(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_uplink);

    OpenProject(ref_uplink, satellite_addr, ref_apikey, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestNewUplink);
    RUN_TEST(TestOpenProject);
    return UNITY_END();
}
