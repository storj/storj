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

    Bytes_t write_data = {
        "test write data 123"
    };
    write_data.length = strlen((char *)write_data.bytes);
    WriteBuffer(ref_buf, &write_data, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    Bytes_t *read_data = malloc(sizeof(Bytes_t));
    ReadBuffer(ref_buf, read_data, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_EQUAL(write_data.length, read_data->length);
    TEST_ASSERT_EQUAL(0, memcmp(write_data.bytes, read_data->bytes, write_data.length));
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestAPIKey);
    RUN_TEST(TestGetIDVersion);
    RUN_TEST(TestBuffer);
    return UNITY_END();
}
