// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include "../uplink-cgo-common.h"

// gcc -o cgo-test-bin lib/uplink/ext/example/main.c lib/uplink/ext/uplink-cgo-common.so

int main() {
//    struct Config uplinkConfig = {{{true, ""}, 3}};
    struct Config uplinkConfig;
    struct IDVersion idVersion = {2,};
//    uplinkConfig.Volatile.IdentityVersion = {2};
    uplinkConfig.Volatile.IdentityVersion = idVersion;
    uplinkConfig.Volatile.TLS.SkipPeerCAWhitelist = true;

    char *err = "";
    struct Uplink uplink = NewUplink(uplinkConfig, err);

    printf("testing 123\n");
    if (err == "") {
        printf("error: %s\n", *err);
    }


    printf("%d\n", uplink.Config.Volatile.IdentityVersion.Number);
    printf("%s\n", uplink.Config.Volatile.TLS.SkipPeerCAWhitelist ? "true" : "false");
    printf("%s\n", uplinkConfig.Volatile.TLS.SkipPeerCAWhitelist ? "true" : "false");
//    kill(getpid(), 10);
}