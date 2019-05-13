// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include "unity.h"
#include "../uplink-cgo.h"

void TestGetIDVersion(void)
{
    char *_err = "";
    char **err = &_err;
    uint8_t idVersionNumber = 0;

    struct GoValue idVersionValue = GetIDVersion(idVersionNumber, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    Unpack(&idVersionValue, err);
    IDVersionProto *idVersion = storj__libuplink__idversion__unpack(NULL, idVersionValue.Size, idVersionValue.Snapshot);

    TEST_ASSERT_EQUAL(0, idVersion->number);
}

void TestAPIKey(void)
{
    char *_err = "";
    char **err = &_err;
    char *keyStr = "HiBryanIDidIt";
    APIKey apikey = ParseAPIKey(keyStr, err);
    char *resultKey = Serialize(apikey);

    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_EQUAL_STRING(keyStr, resultKey);
}
