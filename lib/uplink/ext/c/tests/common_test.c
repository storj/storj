// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include "unity.h"
#include "../../uplink-cgo.h"

void TestGetIDVersion(void)
{
    char *_err = "";
    char **err = &_err;
    uint8_t id_version_number = 0;

    IDVersion_t id_version = GetIDVersion(id_version_number, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    TEST_ASSERT_EQUAL(0, id_version.number);
}

void TestAPIKey(void)
{
    char *_err = "";
    char **err = &_err;
    char *key_str = "test apikey";

    APIKeyRef_t ref_apikey = ParseAPIKey(key_str, err);
    char *result_key = Serialize(ref_apikey);

    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_EQUAL_STRING(key_str, result_key);
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestAPIKey);
    RUN_TEST(TestGetIDVersion);
    return UNITY_END();
}
