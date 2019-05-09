// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <signal.h>
#include <string.h>
#include "unity.h"
#include "../uplink-cgo.h"

uint8_t idVersionNumber = 0;
const struct IDVersion idVersion = {0, 8};
const struct Config testUplinkConfig = {
    {{true, "/whitelist.pem"},
     {0, 8},
     "latest",
     1,
     2}};

void TestNewUplink_config(void)
{
    char *_err = "";
    char **err = &_err;
    struct Config uplinkConfig = testUplinkConfig;

    // NB: ensure we get a valid ID version
    struct IDVersion version = GetIDVersion(idVersionNumber, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, version.GoIDVersion);

    uplinkConfig.Volatile.IdentityVersion = version;
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_EQUAL_UINT8(idVersionNumber, uplinkConfig.Volatile.IdentityVersion.Number);

    struct Uplink uplink = NewUplink(uplinkConfig, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, uplink.GoUplink);
    TEST_ASSERT_TRUE(uplink.Config.Volatile.TLS.SkipPeerCAWhitelist);
    TEST_ASSERT_EQUAL_UINT8(idVersionNumber, uplink.Config.Volatile.IdentityVersion.Number);
    TEST_ASSERT_NOT_EQUAL(0, uplink.Config.Volatile.IdentityVersion.GoIDVersion);
}

struct Uplink *NewTestUplink(char **err)
{
    struct Config uplinkConfig = testUplinkConfig;
    struct IDVersion version = GetIDVersion(idVersionNumber, err);
    uplinkConfig.Volatile.IdentityVersion = version;

    struct Uplink *uplink = malloc(sizeof(struct Uplink));
    *uplink = NewUplink(uplinkConfig, err);
    return uplink;
}

 void TestOpenProject(void)
 {
     char *_err = "";
     char **err = &_err;
     uint8_t idVersionNumber = 0;
     struct IDVersion idVersion = {0, 0};
     struct Config uplinkConfig = {
         {
             {true, "/whitelist.pem"},
              idVersion,
              "latest",
              1, 2
          }
      };
     char *satelliteAddr = "127.0.0.1:7777";
     APIKey apiKey = ParseAPIKey("testapikey", err);
     uint8_t encryptionKey[32];
     struct ProjectOptions opts = {
         {&encryptionKey}
     };

     // NB: ensure we get a valid ID version
     idVersion = GetIDVersion(idVersionNumber, err);
     TEST_ASSERT_EQUAL_STRING("", *err);
     TEST_ASSERT_NOT_EQUAL(0, idVersion.GoIDVersion);

     uplinkConfig.Volatile.IdentityVersion = idVersion;
     TEST_ASSERT_EQUAL_STRING("", *err);
     TEST_ASSERT_EQUAL_UINT8(idVersionNumber, uplinkConfig.Volatile.IdentityVersion.Number);

     struct Uplink *uplink = NewTestUplink(err);
     TEST_ASSERT_EQUAL_STRING("", *err);
     TEST_ASSERT_NOT_NULL(uplink);

      OpenProject(*uplink, satelliteAddr, apiKey, opts, err);
      TEST_ASSERT_EQUAL_STRING("", *err);
 }
