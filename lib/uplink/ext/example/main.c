#include <stdio.h>
#include <unistd.h>
#include "../uplink-cgo-common.h"

// gcc -o cgo-test-bin lib/uplink/ext/main.c lib/uplink/ext/uplink-cgo-common.so

int main() {
//    GoString key = {"butts", 5};
//    struct APIKey apikey = ParseAPIKey(key);
//
//    char *val = Serialize(apikey);
//
//    printf ("apikey = %s\n", val);
//
//    free(val);
//    TestMe()

    struct Config uplinkConfig;
    uplinkConfig.volatile_.IdentityVersion = 1;
    uplinkConfig.volatile_.tls.SkipPeerCAWhitelist = true;

    char *err = "";
    struct Uplink uplink;
    struct Config cfg = NewUplink(uplinkConfig, err);

    printf("testing 123\n");
    if (err == "") {
        printf("error: %s\n", *err);
    }


    printf("%d\n", uplinkConfig.volatile_.IdentityVersion);
//    printf("%s\n", cfg.volatile_.tls);
    printf("%p\n", uplinkConfig.volatile_.tls.SkipPeerCAWhitelist);
    printf("%p\n", cfg.volatile_.tls.SkipPeerCAWhitelist);
//    printf("%p\n", uplink);
//    printf("%p\n", uplink.config);
//    printf("%p\n", uplink.config->volatile_);
//    if (uplink.config == NULL) {
//        printf("uplink.config is null\n");
//    }
//    kill(getpid(), 10);
//    printf("SkipPeerCAWhitelist: %s\n", uplink.config->volatile_.tls.SkipPeerCAWhitelist);
}