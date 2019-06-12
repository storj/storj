// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// TestBucketConfig returns test bucket configuration.
BucketConfig TestBucketConfig() {
    BucketConfig config = {};

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

// WithTestProject opens default test project and calls handleProject callback.
void WithTestProject(void (*handleProject)(Project)) {
    char *_err = "";
    char **err = &_err;

    char *satellite_addr = getenv("SATELLITE_0_ADDR");
    char *apikeyStr = getenv("GATEWAY_0_APIKEY");

    {
        UplinkConfig cfg = {};
        cfg.Volatile.TLS.SkipPeerCAWhitelist = 1; // TODO: add CA Whitelist

        // New uplink
        Uplink uplink = NewUplink(cfg, err);
        require_noerror(*err);
        requiref(uplink._handle != 0, "got empty uplink\n");

        {
            // parse api key
            APIKey apikey = ParseAPIKey(apikeyStr, err);
            require_noerror(*err);
            requiref(apikey._handle != 0, "got empty apikey\n");

            {
                // open a project
                Project project = OpenProject(uplink, satellite_addr, apikey, err);
                require_noerror(*err);
                requiref(project._handle != 0, "got empty project\n");

                handleProject(project);

                // close project
                CloseProject(project, err);
                require_noerror(*err);
            }

            // free api key
            FreeAPIKey(apikey);
        }

        // Close uplinks
        CloseUplink(uplink, err);
        require_noerror(*err);
    }

    requiref(internal_UniverseIsEmpty(), "universe is not empty\n");
}