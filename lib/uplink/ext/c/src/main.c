// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdlib.h>
#include "../../uplink-cgo.h"

//extern void Unpack(struct GoValue *, char **);
//void *UnpackValue(struct GoValue *, char **);

// TODO: move into go?
void *UnpackValue(struct GoValue *val, char **err)
{
    switch (val->Type)
    {
    case IDVersionType:
        Unpack(val, err);
        return (void *)storj__libuplink__idversion__unpack(NULL, val->Size, val->Snapshot);
    default:
        *err = "unknown type";
        return NULL;
    }

    return NULL;
}
