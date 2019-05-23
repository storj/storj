// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <signal.h>
#include <string.h>
#include "unity.h"
#include "../../uplink-cgo.h"

pbUplinkConfig *NewTestConfig(char **err)
{
    uint8_t idVersionNumber = 0;

    // NB: ensure we get a valid ID version
    pbIDVersion idVersion = GetIDVersion(idVersionNumber, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    TEST_ASSERT_EQUAL(idVersionNumber, idVersion->number);

    pbTLSConfig tlsConfig = STORJ__LIBUPLINK__TLSCONFIG__INIT;
    tlsConfig.skip_peer_ca_whitelist = 1;
    tlsConfig.peer_ca_whitelist_path = "/whitelist.pem";

    pbUplinkConfig uplinkConfig = STORJ__LIBUPLINK__UPLINK_CONFIG__INIT;
    uplinkConfig.tls = &tlsConfig;
    uplinkConfig.identity_version = idVersion;
    uplinkConfig.peer_id_version = "latest";
    uplinkConfig.max_inline_size = 1;
    uplinkConfig.max_memory = 2;

    return &uplinkConfig;
}

UplinkRef NewTestUplink(char **err)
{
    pbUplinkConfig *uplinkConfig = NewTestConfig(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_NULL(uplinkConfig);

    return NewUplink(uplinkConfig, err);
}

void TestNewUplink_config(void)
{
    char *_err = "";
    char **err = &_err;

    pbUplinkConfig *uplinkConfig = NewTestConfig(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_NULL(uplinkConfig);

    UplinkRef uplinkRef = NewUplink(uplinkConfig, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, uplinkRef);
}

void TestOpenProject(void)
{
    char *_err = "";
    char **err = &_err;
    char *satelliteAddr = getenv("SATELLITEADDR");
    APIKeyRef apiKey = ParseAPIKey(getenv("APIKEY"), err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    pbProjectOptions opts = STORJ__LIBUPLINK__PROJECT_OPTIONS__INIT;
    // NB: empty encryption key
    uint8_t encryptionKey[32];
    memset(encryptionKey, '\0', sizeof(encryptionKey));
    opts.encryption_key.data = encryptionKey;
    opts.encryption_key.len = sizeof(encryptionKey);

    UplinkRef uplink = NewTestUplink(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, uplink);

    OpenProject(ref_uplink, satellite_addr, ref_apiKey, pb_opts, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
}