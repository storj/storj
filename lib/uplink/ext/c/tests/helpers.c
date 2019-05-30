// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdio.h>
#include "unity.h"
#include "../../uplink-cgo.h"
#include <inttypes.h>

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

EncryptionAccess_t * NewEncryptionAccess(uint8_t *key, int key_len)
{
    EncryptionAccess_t *access = malloc(sizeof(EncryptionAccess_t));
    access->key = malloc(sizeof(Bytes_t));
    access->key->length = key_len;
    access->key->bytes = calloc(key_len, sizeof(uint8_t));

    memcpy(access->key->bytes, key, key_len);

    return access;
}

void freeEncryptionAccess(EncryptionAccess_t *access) {
    if (access != NULL) {
        if (access->key != NULL) {
            if (access->key->bytes != NULL) {
                free(access->key->bytes);
            }
            free(access->key);
        }
        free(access);
    }
}