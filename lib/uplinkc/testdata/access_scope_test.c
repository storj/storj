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

    char *scopeStr = "1ZYMge4erhJ7hSTf4UCUvtcT2e7rHBNrQvVMgxVDPgFwndj2f2tUnoqmQhaQapEvkifiu9Dwi53C8a3QKB8xMYPZkKS3yCLKbhaccpRg91iDGJuUBS7m7FKW2AmvQYNm5EM56AJrCsb95CL4jTd686sJmuGMnpQhd6NqE7bYAsQTCyADUS15kDJ2zBzt43k689TwW";
    {
        ScopeRef scope = parse_scope(scopeStr, err);
        require_noerror(*err);
        requiref(scope._handle != 0, "got empty scope\n");

        char *scopeSerialized = serialize_scope(scope, err);
        require_noerror(*err);

        requiref(strcmp(scopeSerialized, scopeStr) == 0,
                 "got invalid serialized %s expected %s\n", scopeSerialized, scopeStr);

        char *satelliteAddres = get_scope_satellite_address(scope, err);
        require_noerror(*err);
        require(satelliteAddres != NULL);
        require(strcmp(satelliteAddres, "") !=0);
        
        APIKeyRef apikey = get_scope_api_key(scope, err);
        require_noerror(*err);
        requiref(apikey._handle != 0, "got empty api key\n");
        
        EncryptionAccessRef ea = get_scope_enc_access(scope, err);
        require_noerror(*err);
        requiref(ea._handle != 0, "got empty encryption access\n");
        
        ScopeRef newScope = new_scope(satelliteAddres, apikey, ea, err);
        require_noerror(*err);
        requiref(newScope._handle != 0, "got empty scope\n");

        char *newScopeSerialized = serialize_scope(newScope, err);
        require_noerror(*err);

        requiref(strcmp(newScopeSerialized, scopeStr) == 0,
                 "got invalid serialized %s expected %s\n", newScopeSerialized, scopeStr);

        free_scope(scope);
        free_scope(newScope);
        free_api_key(apikey);
        free_encryption_access(ea);
    }

    {
        ScopeRef scope = parse_scope(scopeStr, err);
        require_noerror(*err);
        requiref(scope._handle != 0, "got empty scope\n");

        Caveat caveat = {disallow_writes : true};
        EncryptionRestriction restrictions[] = {
            {"bucket1",
             "path1"},
            {"bucket2",
             "path2"}};

        // invalid restrictionsLen
        ScopeRef restrictedScope = restrict_scope(scope, caveat, &restrictions[0], -1, err);
        require_error(*err);
        *err = "";

        restrictedScope = restrict_scope(scope, caveat, &restrictions[0], 2, err);
        require_noerror(*err);
        requiref(restrictedScope._handle != 0, "got empty scope\n");

        free_scope(scope);
        free_scope(restrictedScope);
    }

    requiref(internal_UniverseIsEmpty(), "universe is not empty\n");
}