// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdlib.h>
#include <time.h>

// test_bucket_config returns test bucket configuration.
BucketConfig test_bucket_config() {
    BucketConfig config = {};

    config.path_cipher = STORJ_ENC_AESGCM;

    config.encryption_parameters.cipher_suite = STORJ_ENC_AESGCM;
    config.encryption_parameters.block_size = 2048;

    config.redundancy_scheme.algorithm = STORJ_REED_SOLOMON;
    config.redundancy_scheme.share_size = 256;
    config.redundancy_scheme.required_shares = 4;
    config.redundancy_scheme.repair_shares = 6;
    config.redundancy_scheme.optimal_shares = 8;
    config.redundancy_scheme.total_shares = 10;

    return config;
}

// with_test_project opens default test project and calls handleProject callback.
void with_test_project(void (*handleProject)(ProjectRef)) {
    char *_err = "";
    char **err = &_err;

    char *satellite_addr = getenv("SATELLITE_0_ADDR");
    char *apikeyStr = getenv("GATEWAY_0_API_KEY");

    printf("using SATELLITE_0_ADDR: %s\n", satellite_addr);
    printf("using GATEWAY_0_API_KEY: %s\n", apikeyStr);

    {
        UplinkConfig cfg = {};
        cfg.Volatile.tls.skip_peer_ca_whitelist = true; // TODO: add CA Whitelist

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
                ProjectRef project = open_project(uplink, satellite_addr, apikey, err);
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

void fill_random_data(uint8_t *buffer, size_t length) {
     for(size_t i = 0; i < length; i++) {
          buffer[i] = (uint8_t)i*31;
     }
}

bool array_contains(char *item, char *array[], int array_size) {
    for (int i = 0; i < array_size; i++) {
        if(strcmp(array[i], item) == 0) {
            return true;
        }
    }

    return false;
}