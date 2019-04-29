#include <stdio.h>
#include "uplink-cgo.h"
//#include "uplink.h"

// gcc -o cgo-test-bin lib/uplink/ext/main.c lib/uplink/ext/uplink-cgo-common.so

int main() {
    GoString key = {"poop", 5};
    struct APIKey apikey = ParseAPIKey(key);

    char *val = Serialize(apikey);

    printf ("apikey = %s\n", val); 

    free(val);
}