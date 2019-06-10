// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>

#include "uplink.h"

void TestGetIDVersion()
{
    char *_err = "";
    char **err = &_err;
    uint8_t id_version_number = 0;

    IDVersion_t id_version = GetIDVersion(id_version_number, err);
    assert(strcmp("", *err) == 0);

    assert(0 == id_version.number);
}

void TestAPIKey()
{
    char *_err = "";
    char **err = &_err;
    char *key_str = "test apikey";

    APIKeyRef_t ref_apikey = ParseAPIKey(key_str, err);
    char *result_key = Serialize(ref_apikey);

    assert(strcmp("", *err) == 0);
    assert(strcmp(key_str, result_key) == 0);
}

int main(int argc, char *argv[])
{
    TestAPIKey();
    TestGetIDVersion();
    return 0;
}
