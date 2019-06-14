// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// test_bucket_config returns test bucket configuration.
BucketConfig_t test_bucket_config() {
    BucketConfig_t config = {};

    config.path_cipher = 0;

    config.encryption_parameters.cipher_suite = 1; // TODO: make a named const
    config.encryption_parameters.block_size = 4096;

    config.redundancy_scheme.algorithm = 1; // TODO: make a named const
    config.redundancy_scheme.share_size = 1024;
    config.redundancy_scheme.required_shares = 2;
    config.redundancy_scheme.repair_shares = 4;
    config.redundancy_scheme.optimal_shares = 5;
    config.redundancy_scheme.total_shares = 6;

    return config;
}

// with_test_project opens default test project and calls handleProject callback.
void with_test_project(void (*handleProject)(ProjectRef_t)) {
    char *_err = "";
    char **err = &_err;

    char *satellite_addr = getenv("SATELLITE_0_ADDR");
    char *apikeyStr = getenv("GATEWAY_0_API_KEY");

    printf("using SATELLITE_0_ADDR: %s\n", satellite_addr);
    printf("using GATEWAY_0_API_KEY: %s\n", apikeyStr);

    {
        UplinkConfig_t cfg = {};
        cfg.Volatile.TLS.SkipPeerCAWhitelist = true; // TODO: add CA Whitelist

        // New uplink
        UplinkRef_t uplink = new_uplink(cfg, err);
        require_noerror(*err);
        requiref(uplink._handle != 0, "got empty uplink\n");

        {
            // parse api key
            APIKeyRef_t apikey = parse_api_key(apikeyStr, err);
            require_noerror(*err);
            requiref(apikey._handle != 0, "got empty apikey\n");

            {
                // open a project
                ProjectRef_t project = open_project(uplink, satellite_addr, apikey, err);
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