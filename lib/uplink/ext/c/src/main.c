// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdlib.h>
#include <string.h>
#include "../../uplink-cgo.h"

// get_snapshot gets the values from the GoValue->Ptr struct and convert them into a protobuf for C code to read
void *get_snapshot(struct GoValue *val, char **err)
{
    if (val->Ptr == 0)
    {
        *err = "empty ptr error: go value was created in C";
        return NULL;
    }

    switch (val->Type)
    {
    case IDVersionType:
        CGetSnapshot(val, err);
        return (void *)storj__libuplink__idversion__unpack(NULL, val->Size, val->Snapshot);
    default:
        *err = "unknown type";
        return NULL;
    }

    return NULL;
}

// protoToGoValue takes a protobuf, serializes it, sends it to go code, the go code converts that into a go struct and stores it
void protoToGoValue(void *proto_msg, enum ValueType value_type, struct GoValue *value, char **err)
{
    // Serialize the protobuf into the value
    switch (value_type)
    {
    case UplinkConfigType:
        value->Size = storj__libuplink__uplink_config__get_packed_size((pbUplinkConfig *)proto_msg);
        value->Snapshot = malloc(value->Size);
        value->Type = value_type;
        storj__libuplink__uplink_config__pack((pbUplinkConfig *)proto_msg, value->Snapshot);
        break;
    default:
        *err = "unknown type";
        return;
    }

    SendToGo(value, err);
    return;
}
