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

void TestBuffer(void)
{
    char *_err = "";
    char **err = &_err;

    BufferRef_t ref_buf = NewBuffer();

    char *write_data = "test data 123";
    WriteBuffer(ref_buf, (uint8_t *)write_data, sizeof(write_data), err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    size_t data_size;
    uint8_t *read_data;
    ReadBuffer(ref_buf, &read_data, &data_size, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, data_size);
    TEST_ASSERT_EQUAL(0, strcmp((char *)write_data, (char *)read_data));
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestAPIKey);
    RUN_TEST(TestGetIDVersion);
//    RUN_TEST(TestBuffer);
    return UNITY_END();
}
