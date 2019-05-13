// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdlib.h>
#include "../headers/main.h"


void* ConvertValue(struct GoValue *val, char **err)
{
    switch(val->Type) {
    case IDVersionType:
//        IDVersionProto *idVersionProto = storj__libuplink__idversion__unpack(NULL, val->Size, val->Snapshot);

//        struct IDVersion idVersion = {IDVersionProto.number};
        break;
    default:
        *err = "unknown type";
        return NULL;
    }
}