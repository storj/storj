// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include "unity.h"
#include "../../uplink-cgo.h"

void TestGetIDVersion(void)
{
    char *_err = "";
    char **err = &_err;
    uint8_t idVersionNumber = 0;

    gvIDVersion idVersionValue = GetIDVersion(idVersionNumber, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    IDVersion *idVersion = (IDVersion *)(get_snapshot(&idVersionValue, err));
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_NULL(idVersion);

    TEST_ASSERT_EQUAL(0, idVersion->number);
}

void TestAPIKey(void)
{
    char *_err = "";
    char **err = &_err;
    char *keyStr = "HiBryanIDidIt";
    gvAPIKey apikey = ParseAPIKey(keyStr, err);
    char *resultKey = Serialize(apikey.Ptr);

    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_EQUAL_STRING(keyStr, resultKey);
}
