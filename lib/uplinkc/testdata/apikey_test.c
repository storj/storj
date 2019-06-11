// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include <time.h>
#include <stdlib.h>

#include "require.h"
#include "uplink.h"

int main(int argc, char *argv[])
{
    char *_err = "";
    char **err = &_err;

    APIKey apikey = ParseAPIKey("testapikey123", err);
    require_noerror(*err);

    char *apikeyserialized = SerializeAPIKey(apikey, err);
    require_noerror(*err);
    require(strcmp(apikeyserialized, "testapikey123") == 0, "got different serialized %s\n", apikeyserialized);
    free(apikeyserialized);

    FreeAPIKey(apikey, err);
    require_noerror(*err);

    IDVersion version = GetIDVersion(0, err);
    require_noerror(*err);
    require(version.number == 0, "invalid version number, got %d\n", version.number);

    require(internal_UniverseIsEmpty(), "universe is not empty\n");
}
