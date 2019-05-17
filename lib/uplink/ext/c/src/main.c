// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdlib.h>
#include <string.h>
#include "../../uplink-cgo.h"

// TODO: move into go?
void *unpack_value(struct GoValue *val, char **err)
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

void pack_value(void *proto_msg, enum ValueType value_type, struct GoValue *value, char **err)
{
    switch (value_type)
    {
    case IDVersionType:
        value->Size = storj__libuplink__idversion__pack((IDVersion *)proto_msg, value->Snapshot);
    case UplinkConfigType:
        value->Size = storj__libuplink__uplink_config__pack((UplinkConfig *)proto_msg, value->Snapshot);
    default:
        *err = "unknown type";
        return;
    }

    Pack(value, err);
    if (strcmp("", *err) != 0) {
        return;
    }

    return;
}
