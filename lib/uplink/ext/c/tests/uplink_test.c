// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <signal.h>
#include <string.h>
#include "unity.h"
#include "../../uplink-cgo.h"

gvUplinkConfig *NewTestConfig(char **err)
{
    uint8_t idVersionNumber = 0;

    // NB: ensure we get a valid ID version
    gvIDVersion idVersionValue = GetIDVersion(idVersionNumber, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    pbIDVersion *idVersion = (pbIDVersion *)(get_snapshot(&idVersionValue, err));
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_NULL(idVersion);

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

    gvUplinkConfig *uplinkConfigValue = malloc(sizeof(gvUplinkConfig));
    uplinkConfigValue->Type = UplinkConfigType;
    protoToGoValue(&uplinkConfig, uplinkConfigValue, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

//    pbUplinkConfig *uplinkConfig2 = (get_snapshot(uplinkConfigValue, err));
//    TEST_ASSERT_EQUAL_STRING("", *err);

    pbUplinkConfig *uplinkConfig2 = storj__libuplink__uplink_config__unpack(NULL, uplinkConfigValue->Size, uplinkConfigValue->Snapshot);
    TEST_ASSERT_EQUAL_STRING("", *err);
//    printf("")
    TEST_ASSERT_EQUAL_STRING(uplinkConfig.peer_id_version, uplinkConfig2->peer_id_version);
    TEST_ASSERT_EQUAL(uplinkConfig.max_inline_size, uplinkConfig2->max_inline_size);
    TEST_ASSERT_EQUAL(uplinkConfig.max_memory, uplinkConfig2->max_memory);

    return uplinkConfigValue;
}

UplinkRef NewTestUplink(char **err)
{
    gvUplinkConfig *uplinkConfigValue = NewTestConfig(err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    return NewUplink(uplinkConfigValue->Ptr, err);
}

void TestNewUplink_config(void)
{
    char *_err = "";
    char **err = &_err;

    gvUplinkConfig *uplinkConfigValue = NewTestConfig(err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    UplinkRef uplinkRef = NewUplink(uplinkConfigValue->Ptr, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, uplinkRef);
}

void TestOpenProject(void)
{
    char *_err = "";
    char **err = &_err;
    char *satelliteAddr = getenv("SATELLITEADDR");
    gvAPIKey apiKey = ParseAPIKey(getenv("APIKEY"), err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    pbProjectOptions opts = STORJ__LIBUPLINK__PROJECT_OPTIONS__INIT;
    // NB: empty encryption key
    uint8_t encryptionKey[32];
    memset(encryptionKey, '\0', sizeof(encryptionKey));
    opts.encryption_key.data = encryptionKey;
    opts.encryption_key.len = sizeof(encryptionKey);

    gvProjectOptions *optsValue = malloc(sizeof(gvProjectOptions));
    optsValue->Type = ProjectOptionsType;
    protoToGoValue(&opts, optsValue, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    UplinkRef uplinkRef = NewTestUplink(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, uplinkRef);

    OpenProject(uplinkRef, satelliteAddr, apiKey.Ptr, *optsValue, err);
    TEST_ASSERT_EQUAL_STRING("", *err);
}