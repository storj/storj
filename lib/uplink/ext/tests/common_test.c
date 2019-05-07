// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include "unity.h"
#include "../uplink-cgo.h"

// gcc -o cgo-test-bin lib/uplink/ext/tests/*.c lib/uplink/ext/uplink-cgo.so

void TestGetIDVersion(void) {
    char *err = "";
    uint8_t idVersionNumber = 0;
    struct IDVersion idVersion = {0, 0};

    idVersion = GetIDVersion(idVersionNumber, &err);

    TEST_ASSERT_EQUAL_STRING("", err);
    TEST_ASSERT_NOT_EQUAL(0, idVersion.GoIDVersion);
}
