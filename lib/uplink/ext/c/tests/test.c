// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// gcc -o cgo-test-bin lib/uplink/ext/c/src/*.c lib/uplink/ext/c/pb/*.c lib/uplink/ext/tests/{test,unity,*_test}.c lib/uplink/ext/c/headers/uplink-cgo.so

#include <stdio.h>
#include <unistd.h>
#include "unity.h"
#include "../../uplink-cgo.h"

extern void TestGetIDVersion(void);
extern void TestAPIKey(void);
extern void TestNewUplink(void);
extern void TestOpenProject(void);
extern void TestCreateBucket(void);
extern void TestOpenBucket(void);
extern void TestValue(void);

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestAPIKey);
    RUN_TEST(TestGetIDVersion);
    RUN_TEST(TestNewUplink);
    RUN_TEST(TestOpenProject);
    RUN_TEST(TestCreateBucket);
//    RUN_TEST(TestOpenBucket);
    return UNITY_END();
}
