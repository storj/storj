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
            APIKey apikey = parse_api_key(apikeyStr, err);
            require_noerror(*err);
            requiref(apikey._handle != 0, "got empty apikey\n");

            {
                // open a project
                Project project = OpenProject(uplink, satellite_addr, apikey, err);
                require_noerror(*err);
                requiref(project._handle != 0, "got empty project\n");

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