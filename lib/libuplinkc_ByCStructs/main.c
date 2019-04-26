#include <stdio.h>
#include "awesome.h"

// gcc -o client main.c ./awesome.so

int main() {
    GoString key = {"butts", 5};
    struct APIKey apikey = ParseAPIKey(key);

    char *val = Serialize(apikey);

    printf ("apikey = %s\n", val); 

    free(val);
}