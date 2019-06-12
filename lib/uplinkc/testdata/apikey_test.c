// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdlib.h>

#include "require.h"

#include "uplink.h"

int main(int argc, char *argv[])
{
    char *_err = "";
    char **err = &_err;

    char *apikeyStr = "test123123";

    {
        // parse api key
        APIKey apikey = ParseAPIKey(apikeyStr, err);
        require_noerror(*err);
        requiref(apikey._handle != 0, "got empty apikey\n");

        char *apikeySerialized = SerializeAPIKey(apikey);
        requiref(strcmp(apikeySerialized, apikeyStr) == 0,
            "got invalid serialized %s expected %s\n", apikeySerialized, apikeyStr);
        free(apikeySerialized);

        // free api key
        FreeAPIKey(apikey);
    }

    requiref(internal_UniverseIsEmpty(), "universe is not empty\n");
}