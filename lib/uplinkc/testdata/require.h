#include <assert.h>
#include <stdio.h>

#define require(test, msg, ...) \
do { \
    if(!(test)) { \
        printf(msg, ##__VA_ARGS__);\
        printf("failed:\n\t%s:%d: %s", __FILE__, __LINE__, #test);\
        exit(1);\
    }\
} while (0)

#define require_noerror(err) \
do { \
    if(strcmp("", err) != 0) { \
        printf("failed:\n\t%s:%d: %s", __FILE__, __LINE__, err);\
        exit(1);\
    }\
} while (0)
