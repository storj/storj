// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include "unity.h"
#include "../../uplink-cgo.h"


void TestOpenBucket(void)
{
    TEST_ASSERT_EQUAL_STRING("", "");
}

int main(int argc, char *argv[])
{
    UNITY_BEGIN();
    RUN_TEST(TestOpenBucket);
    return UNITY_END();
}
