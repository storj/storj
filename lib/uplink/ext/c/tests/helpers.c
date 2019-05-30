// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdio.h>
#include "unity.h"
#include "../../uplink-cgo.h"

ProjectRef_t OpenTestProject(char **err)
{
    char *satellite_addr = getenv("SATELLITE_ADDR");
    APIKeyRef_t ref_apikey = ParseAPIKey(getenv("APIKEY"), err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    UplinkRef_t ref_uplink = NewUplinkInsecure(err);
    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_NOT_EQUAL(0, ref_uplink);

    return OpenProject(ref_uplink, satellite_addr, ref_apikey, err);
}

// TODO: fix this
//NewEncryptionAccess(uint8_t *key, EncryptionAccess_t *access)
//{
//    Bytes_t key_bytes;
//    key_bytes.bytes = key;
//    // NB: only works with null terminated arrays
//    key_bytes.length = strlen((const char *)key);
//    EncryptionAccess_t _access = {&key_bytes};
//    *access = _access;
////    access->key = &key_bytes;
//    printf("key_bytes.bytes %p\n", key_bytes.bytes);
//}
