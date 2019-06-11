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
    char *apikey = getenv("GATEWAY_0_APIKEY");

    {
        // New uplink
        Uplink uplink = NewUplinkInsecure(err); // TODO: replace with NewUplink
        require_noerror(*err);
        require(uplink._ref != 0, "got empty uplink\n");

        // open a project
        Project project = OpenProject(uplink, satellite_addr, apikey, err);
        require_noerror(*err);

        // close project
        CloseProject(project, err);
        require_noerror(*err);

        // Close uplinks
        CloseUplink(uplink, err);
        require_noerror(*err);
    }

    require(internal_UniverseIsEmpty(), "universe is not empty\n");
}