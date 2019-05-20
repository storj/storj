// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdlib.h>
#include <string.h>
#include "../../uplink-cgo.h"

// TODO: move into go?
void *get_snapshot(struct GoValue *val, char **err)
{
    switch (val->Type)
    {
    case IDVersionType:
        GetSnapshot(val, err);
        return (void *)storj__libuplink__idversion__unpack(NULL, val->Size, val->Snapshot);
    default:
        *err = "unknown type";
        return NULL;
    }

    return NULL;
}

void protoToGoValue(void *proto_msg, enum ValueType value_type, struct GoValue *value, char **err)
{

    switch (value_type)
    {
    case UplinkConfigType:
        value->Size = storj__libuplink__uplink_config__get_packed_size((UplinkConfig *)proto_msg);
        value->Snapshot = malloc(value->Size);
        value->Type = value_type;
        storj__libuplink__uplink_config__pack((UplinkConfig *)proto_msg, value->Snapshot);
        //        printf("value->Snapshot: %p\n", value->Snapshot);
        //        printf("value->Snapshot: %d\n", value->Snapshot[0]);
        break;
    default:
        *err = "unknown type";
        return;
    }

    SendToGo(value, err);
    if (strcmp("", *err) != 0)
    {
        return;
    }

    return;
}
