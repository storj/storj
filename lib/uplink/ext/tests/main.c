// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// gcc -o cgo-test-bin lib/uplink/ext/tests/{main,unity,*_test}.c lib/uplink/ext/uplink-cgo.so

#include <stdio.h>
#include <unistd.h>
#include "unity.h"
#include "../uplink-cgo.h"

extern void TestNewUplink_config(void);
extern void TestGetIDVersion(void);

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(TestNewUplink_config);
    RUN_TEST(TestGetIDVersion);
    return UNITY_END();
}
