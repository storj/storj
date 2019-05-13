// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <signal.h>
#include <string.h>
#include "unity.h"
#include "../uplink-cgo.h"

void TestNewUplink_config(void)
{
    uint8_t idVersionNumber = 0;
    char *_err = "";
    char **err = &_err;

    // NB: ensure we get a valid ID version
//    struct IDVersion version = GetIDVersion(idVersionNumber, err);
//    TEST_ASSERT_EQUAL_STRING("", *err);
//
//    struct Config testUplinkConfig = {
//        {{true, "/whitelist.pem"},
//         version,
//         "latest",
//         1,
//         2}};
//
//    testUplinkConfig.Volatile.IdentityVersion = version;
//    TEST_ASSERT_EQUAL_STRING("", *err);
//
//    struct Uplink uplink = NewUplink(testUplinkConfig, err);
//    TEST_ASSERT_EQUAL_STRING("", *err);
//    TEST_ASSERT_NOT_EQUAL(0, uplink.GoUplink);
//    TEST_ASSERT_TRUE(uplink.Config.Volatile.TLS.SkipPeerCAWhitelist);
}

struct Uplink *NewTestUplink(char **err)
{
    uint8_t idVersionNumber = 0;
//    IDVersionPtr version = GetIDVersion(idVersionNumber, err);
//
//    struct Config testUplinkConfig = {
//        {{true, "/whitelist.pem"},
//         version,
//         "latest",
//         1,
//         2}};
//
//    struct Uplink *uplink = malloc(sizeof(struct Uplink));
//    *uplink = NewUplink(testUplinkConfig, err);
//    return uplink;
}

void TestOpenProject(void)
{
    char *_err = "";
    char **err = &_err;
    char *satelliteAddr = getenv("SATELLITEADDR");
    APIKey apiKey = ParseAPIKey(getenv("APIKEY"), err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    uint8_t encryptionKey[32];
    struct ProjectOptions opts = {
        {&encryptionKey}
    };

    struct Uplink *uplink = NewTestUplink(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_NULL(uplink);

    OpenProject(*uplink, satelliteAddr, apiKey, opts, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
}
