// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdlib.h>
#include <time.h>

// test_bucket_config returns test bucket configuration.
BucketConfig test_bucket_config() {
    BucketConfig config = {};

    config.path_cipher = 0;

    config.encryption_parameters.cipher_suite = 1; // TODO: make a named const
    config.encryption_parameters.block_size = 2048;

    config.redundancy_scheme.algorithm = 1; // TODO: make a named const
    config.redundancy_scheme.share_size = 1024;
    config.redundancy_scheme.required_shares = 2;
    config.redundancy_scheme.repair_shares = 4;
    config.redundancy_scheme.optimal_shares = 5;
    config.redundancy_scheme.total_shares = 6;

    return config;
}

// with_test_project opens default test project and calls handleProject callback.
void with_test_project(void (*handleProject)(ProjectRef), ProjectOptions *project_opts) {
    char *_err = "";
    char **err = &_err;

    char *satellite_addr = getenv("SATELLITE_0_ADDR");
    char *apikeyStr = getenv("GATEWAY_0_API_KEY");

    printf("using SATELLITE_0_ADDR: %s\n", satellite_addr);
    printf("using GATEWAY_0_API_KEY: %s\n", apikeyStr);

    {
        UplinkConfig cfg = {};
        cfg.Volatile.TLS.SkipPeerCAWhitelist = true; // TODO: add CA Whitelist

        // New uplink
        UplinkRef uplink = new_uplink(cfg, err);
        require_noerror(*err);
        requiref(uplink._handle != 0, "got empty uplink\n");

        {
            // parse api key
            APIKeyRef apikey = parse_api_key(apikeyStr, err);
            require_noerror(*err);
            requiref(apikey._handle != 0, "got empty apikey\n");

            {
                // open a project
                ProjectRef project = open_project(uplink, satellite_addr, apikey, project_opts, err);
                require_noerror(*err);
                requiref(project._handle != 0, "got empty project\n");

                handleProject(project);

                // close project
                close_project(project, err);
                require_noerror(*err);
            }

            // free api key
            free_api_key(apikey);
        }

        // Close uplinks
        close_uplink(uplink, err);
        require_noerror(*err);
    }

    requiref(internal_UniverseIsEmpty(), "universe is not empty\n");
}

char *mkrndstr(size_t length) { // const size_t length, supra

    static char charset[] = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789,.-#'?!"; // could be const
    char *randomString = NULL;

    if (length > 0) {
        randomString = malloc(length +1); // sizeof(char) == 1, cf. C99

        if (randomString) {
            int l = (int) (sizeof(charset) -1); // (static/global, could be const or #define SZ, would be even better)
            int key;  // one-time instantiation (static/global would be even better)
            for (int n = 0;n < length;n++) {
                key = rand() % l;   // no instantiation, just assignment, no overhead from sizeof
                randomString[n] = charset[key];
            }

            randomString[length] = '\0';
        }
    }

    return randomString;
}

bool array_contains(char *item, char *array[], int array_size) {
    for (int i = 0; i < array_size; i++) {
        if(strcmp(array[i], item) == 0) {
            return true;
        }
    }

    return false;
}