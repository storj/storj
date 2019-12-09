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

        free_scope(scope);
    }
    { //restrict_scope - checks scope
	ScopeRef scope; //just need a valid scope-variable for this test 
	Caveat caveat; //just need a valid caveat-variable for this test
        ScopeRef emptyScope = restrict_scope(scope, caveat, NULL, err);
	requiref(strcmp("invalid scope", err) == 0,
                 "Scope is not checked from restrict_scope\n");
	requiref(emptyScope._handle == 0, "got no empty scope from restrict_scope if base-scope is invalid\n");
    }

    requiref(internal_UniverseIsEmpty(), "universe is not empty\n");
}
