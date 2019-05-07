// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include "unity.h"
#include "../uplink-cgo.h"

extern void TestGetIDVersion(void);

// gcc -o cgo-test-bin lib/uplink/ext/tests/*.c lib/uplink/ext/uplink-cgo.so

void TestNewUplink_config(void) {
    char *err = "";
    uint8_t idVersionNumber = 0;
    struct TLS tls = {true, "/whitelist.pem"};
    struct IDVersion idVersion = {0, 0};
    struct Config uplinkConfig = {
        {
            tls,
            idVersion,
            "latest",
            1, 2
        }
    };

    // NB: ensure we get a valid ID version
    idVersion = GetIDVersion(idVersionNumber, &err);
    TEST_ASSERT_EQUAL_STRING("", err);
    TEST_ASSERT_NOT_EQUAL(0, idVersion.GoIDVersion);

    uplinkConfig.Volatile.IdentityVersion = idVersion;
    TEST_ASSERT_EQUAL_STRING("", err);
    TEST_ASSERT_EQUAL_UINT8(idVersionNumber, uplinkConfig.Volatile.IdentityVersion.Number);

    struct Uplink uplink = NewUplink(uplinkConfig, &err);
    TEST_ASSERT_EQUAL_STRING("", err);

    TEST_ASSERT_TRUE(uplink.Config.Volatile.TLS.SkipPeerCAWhitelist);
    TEST_ASSERT_EQUAL_UINT8(idVersionNumber, uplink.Config.Volatile.IdentityVersion.Number);
    TEST_ASSERT_NOT_EQUAL(0, uplink.Config.Volatile.IdentityVersion.GoIDVersion);
}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(TestNewUplink_config);
    RUN_TEST(TestGetIDVersion);
    return UNITY_END();
}
