// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdlib.h>

#include "require.h"

#include "uplink.h"

//secret: test123: key: 13YqeKQiA3ANSuDu4rqX6eGs3YWox9GRi9rEUKy1HidXiNNm6a5SiE49Hk9gomHZVcQhq4eFQh8yhDgfGKg268j6vqWKEhnJjFPLqAP
//secret: test234; key: 13YqeLP7QuKu1pJVGE3uLNFvdmjE9wZz3c9K2thQWFRpVnjqh3F2qPYsgy6rZaHgJrJFR1vU8nLSyS2xdCdhtKQuL1Kmuh98jao6RCC
//secret: test345; key: 13YqfqzK46AHf53QEnuo8T7q2nERWuhr3j5aQGbW4yz3cDHgtD5ducmfptvGeivqoPLSEtn1HvdDx7NT1VuTFg6EDCvVXZuSBHgcgCr
//secret: test456; key: 13Yqg72PvsxNuoWi4NXoj7GCj5WTTo8bvhF7mg4Z5rkf6VCzC9NECWHCH9o7f2znu71TDrwxPV5Jx6Y8vuarnxJ4peVv6gk7vLjaQBV
//secret: test567; key: 13YqfeyxukbmuyCFsjEqgymiFKprLcS5QHnAFZwus3PREwqRNqz2JktiqhVySJWGMGBbWLWGxDtgULZgZTAAKzeq8gyK3jYuee2hx2D
//secret: test678; key: 3YqdZZ2jLr7QA3pTwaJQvzmXCuwWy7oGbwP6qxXUkxQewo13gn4ZodeYDs27anW7V8cdhUDiddZWvEZGUNhiWPNJH1T9QY3Lajf6RC
//secret: test789; key: 3YqdBGhE7zSBvVfNauGubLP96CDYVSetTTJk8tA15VYTZQ2qZEfiqtK7ycPpoh7xGvN6gn5ky8TDwyrLZ8oTthuhoDTWRFfRnBnCDV

int main(int argc, char *argv[])
{
    char *_err = "";
    char **err = &_err;

    char *apikeyStr = "13YqeKQiA3ANSuDu4rqX6eGs3YWox9GRi9rEUKy1HidXiNNm6a5SiE49Hk9gomHZVcQhq4eFQh8yhDgfGKg268j6vqWKEhnJjFPLqAP";

    {
        // parse api key
        APIKeyRef_t apikey = parse_api_key(apikeyStr, err);
        require_noerror(*err);
        requiref(apikey._handle != 0, "got empty apikey\n");

        char *apikeySerialized = serialize_api_key(apikey, err);
        require_noerror(*err);
        requiref(strcmp(apikeySerialized, apikeyStr) == 0,
            "got invalid serialized %s expected %s\n", apikeySerialized, apikeyStr);
        free(apikeySerialized);

        // free api key
        free_api_key(apikey);
    }

    requiref(internal_UniverseIsEmpty(), "universe is not empty\n");
}