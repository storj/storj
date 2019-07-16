//#include <microhttpd.h>
#include <assert.h>
#include <stdlib.h>

#include "../src/storj.h"
#include "../../uplinkc/testdata/require.h"
//#include "../src/bip39.h"
//#include "../src/utils.h"
//#include "../src/crypto.h"
//
//#include "mockbridge.json.h"
//#include "mockbridgeinfo.json.h"

#define require_no_last_error \
if (strcmp("", *STORJ_LAST_ERROR) != 0) { \
    printf("STORJ_LAST_ERROR: %s\n", *STORJ_LAST_ERROR); \
} \
require(strcmp("", *STORJ_LAST_ERROR) == 0)\

#define require_no_last_error_if(status) \
if (status > 0) { \
    printf("ERROR: %s\n", storj_strerror(status)); \
} \
if (strcmp("", *STORJ_LAST_ERROR) != 0) { \
    printf("STORJ_LAST_ERROR: %s\n", *STORJ_LAST_ERROR); \
} \
require(strcmp("", *STORJ_LAST_ERROR) == 0 && status == 0)\

#define require_not_empty(str) \
require(str != NULL); \
require(strcmp("", str) != 0) \

#define require_equal(str1, str2) \
require(str1 != NULL); \
require(str2 != NULL); \
require(strcmp(str1, str2) == 0) \


#define KRED  "\x1B[31m"
#define KGRN  "\x1B[32m"
#define RESET "\x1B[0m"

////#define USER "testuser@storj.io"
////#define PASS "dce18e67025a8fd68cab186e196a9f8bcca6c9e4a7ad0be8a6f5e48f3abd1b04"
////#define PASSHASH "83c2db176985cb39d2885b15dc3d2afc020bd886ffee10e954a5848429c03c6d"
////
////int mock_bridge_server(void *cls,
////                       struct MHD_Connection *connection,
////                       const char *url,
////                       const char *method,
////                       const char *version,
////                       const char *upload_data,
////                       size_t *upload_data_size,
////                       void **ptr);
////
////int mock_farmer_shard_server(void *cls,
////                             struct MHD_Connection *connection,
////                             const char *url,
////                             const char *method,
////                             const char *version,
////                             const char *upload_data,
////                             size_t *upload_data_size,
////                             void **ptr);
////
////struct MHD_Daemon *start_farmer_server();
////void free_farmer_data();
////
////int create_test_file(char *file);
