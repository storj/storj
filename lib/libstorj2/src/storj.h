/**
 * @file storj.h
 * @brief Storj library.
 *
 * Implements functionality to upload and download files from the Storj
 * distributed network.
 */

#ifndef STORJ_H
#define STORJ_H

#ifdef __cplusplus
extern "C" {
#endif

#if defined(_WIN32) && defined(STORJDLL)
  #if defined(DLL_EXPORT)
    #define STORJ_API __declspec(dllexport)
  #else
    #define STORJ_API __declspec(dllimport)
  #endif
#else
  #define STORJ_API
#endif

#include <json-c/json.h>
#include <stdlib.h>
#include <stdio.h>
#include <stdbool.h>
#include <stdarg.h>
#include <string.h>
#include <uv.h>
#include "uplink.h"

#include <inttypes.h>

#include <time.h>

#ifndef _WIN32
#include <sys/mman.h>
#include <unistd.h>
#endif

// File transfer success
#define STORJ_TRANSFER_OK 0
#define STORJ_TRANSFER_CANCELED 1

// Libuplink error (i.e. check STORJ_LAST_ERROR)
#define STORJ_LIBUPLINK_ERROR 1000

// Memory related errors
#define STORJ_MEMORY_ERROR 2000

// File related errors 3000 to 3999
#define STORJ_FILE_INTEGRITY_ERROR 3000

// Queue related errors
#define STORJ_QUEUE_ERROR 4000

#define STORJ_SHARD_CHALLENGES 4
#define STORJ_LOW_SPEED_LIMIT 30720L
#define STORJ_LOW_SPEED_TIME 20L
#define STORJ_HTTP_TIMEOUT 60L

#define STORJ_DEFAULT_UPLOAD_BUFFER_SIZE (size_t)(32 * 1024 * sizeof(char))
#define STORJ_DEFAULT_DOWNLOAD_BUFFER_SIZE (size_t)(32 * 1024 * sizeof(char))

#define STORJ_RETURN_IF_LAST_ERROR(value) \
if (strcmp("", *STORJ_LAST_ERROR) != 0) { \
    return value;\
}\

#define STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR \
if (strcmp("", *STORJ_LAST_ERROR) != 0) { \
    state->error_status = STORJ_LIBUPLINK_ERROR; \
    return;\
}\

// TODO: should req->status_code be an http error status code?
// (look into how req->status_code is used)
#define STORJ_RETURN_SET_REQ_ERROR_IF_LAST_ERROR \
if (strcmp("", *STORJ_LAST_ERROR) != 0) { \
    req->error_code = STORJ_LIBUPLINK_ERROR; \
    req->status_code = 1; \
    return;\
}\

// TODO: do we need `extern`?
extern char **STORJ_LAST_ERROR;

typedef enum {
    STORJ_REPORT_NOT_PREPARED = 0,
    STORJ_REPORT_AWAITING_SEND = 1,
    STORJ_REPORT_SENDING = 2,
    STORJ_REPORT_SENT = 3
} exchange_report_status_t;

/** @brief Bridge configuration options
 *
 * Proto can be "http" or "https", and the user/pass are used for
 * basic authentication to a Storj bridge.
 */
typedef struct {
    char *addr;
    // NB: apikey is project-specific.
    char *apikey;

} storj_bridge_options_t;

/** @brief File encryption options
 *
 * The mnemonic is a BIP39 secret code used for generating keys for file
 * encryption and decryption.
 */
typedef struct storj_encrypt_options {
    uint8_t key[32];
} storj_encrypt_options_t;



/** @brief HTTP configuration options
 *
 * Settings for making HTTP requests
 */
typedef struct storj_http_options {
    const char *user_agent;
    const char *proxy_url;
    const char *cainfo_path;
    uint64_t low_speed_limit;
    uint64_t low_speed_time;
    uint64_t timeout;
} storj_http_options_t;

/** @brief A function signature for logging
 */
typedef void (*storj_logger_fn)(const char *message, int level, void *handle);

/** @brief Logging configuration options
 *
 * Settings for logging
 */
typedef struct storj_log_options {
    storj_logger_fn logger;
    int level;
} storj_log_options_t;

/** @brief A function signature for logging
 */
typedef void (*storj_logger_format_fn)(storj_log_options_t *options,
                                       void *handle,
                                       const char *format, ...);

/** @brief Functions for all logging levels
 */
typedef struct storj_log_levels {
    storj_logger_format_fn debug;
    storj_logger_format_fn info;
    storj_logger_format_fn warn;
    storj_logger_format_fn error;
} storj_log_levels_t;

/** @brief A structure for a Storj user environment.
 *
 * This is the highest level structure and holds many commonly used options
 * and the event loop for queuing work.
 */
typedef struct storj_env {
    storj_bridge_options_t *bridge_options;
    storj_encrypt_options_t *encrypt_options;
    storj_log_options_t *log_options;

    uv_loop_t *loop;
    uv_async_t async;
    storj_log_levels_t *log;

    /* New in V3 */
    UplinkRef uplink_ref;
    ProjectRef project_ref;

    // TODO: delete?
    /* unused in V3 */
    storj_http_options_t *http_options;
} storj_env_t;

/** @brief A structure for that describes a bucket
 */
typedef struct {
    const char *created;
    const char *name;
    const char *id;
    bool decrypted;
} storj_bucket_meta_t;

/** @brief A structure for queueing create bucket request work
 */
typedef struct {
    ProjectRef project_ref;
    const char *bucket_name;
    BucketConfig *bucket_cfg;
    struct json_object *response;
    storj_bucket_meta_t *bucket;
    int error_code;
    int status_code;
    void *handle;
} create_bucket_request_t;

/** @brief A structure for queueing list buckets request work
 */
typedef struct {
    ProjectRef project_ref;
    struct json_object *response;
    storj_bucket_meta_t *buckets;
    uint32_t total_buckets;
    int error_code;
    int status_code;
    void *handle;
} get_buckets_request_t;

/** @brief A structure for queueing get bucket request work
 */
typedef struct {
    ProjectRef project_ref;
    const char *bucket_name;
    storj_bucket_meta_t *bucket;
    struct json_object *response;
    int error_code;
    int status_code;
    void *handle;
} get_bucket_request_t;

/** @brief A structure for queueing get bucket id request work
 */
typedef struct {
    ProjectRef project_ref;
    const char *bucket_name;
    struct json_object *response;
    const char *bucket_id;
    int error_code;
    int status_code;
    void *handle;
} get_bucket_id_request_t;

/** @brief A structure for queueing delete bucket request work
 */
typedef struct {
    ProjectRef project_ref;
    const char *bucket_name;
    struct json_object *response;
    int error_code;
    int status_code;
    void *handle;
} delete_bucket_request_t;

/** @brief A structure for that describes a bucket entry/file
 */
typedef struct {
    const char *created;
    const char *filename;
    const char *mimetype;
    uint64_t size;
    const char *id;
    const char *bucket_id;
    bool decrypted;
} storj_file_meta_t;

/** @brief A structure for queueing list files request work
 */
typedef struct {
    ProjectRef project_ref;
    const char *encryption_access;
    const char *bucket_id;
    struct json_object *response;
    storj_file_meta_t *files;
    uint32_t total_files;
    int error_code;
    int status_code;
    void *handle;
} list_files_request_t;

/** @brief A structure for queueing get file info request work
 */
typedef struct {
    BucketRef bucket_ref;
    const char *bucket_id;
    char *path;
    struct json_object *response;
    storj_file_meta_t *file;
    int error_code;
    int status_code;
    void *handle;
} get_file_info_request_t;

/** @brief A structure for queueing get file id request work
 */
typedef struct {
    storj_http_options_t *http_options;
    storj_encrypt_options_t *encrypt_options;
    storj_bridge_options_t *options;
    const char *bucket_id;
    const char *file_name;
    struct json_object *response;
    const char *file_id;
    int error_code;
    int status_code;
    void *handle;
} get_file_id_request_t;

/** @brief A structure for queueing delete file request work
 */
typedef struct {
    ProjectRef project_ref;
    const char *bucket_id;
    const char *path;
    const char *encryption_access;
    struct json_object *response;
    int error_code;
    int status_code;
    void *handle;
} delete_file_request_t;

typedef enum {
    BUCKET_PUSH,
    BUCKET_PULL
} storj_bucket_op_t;

static const char *BUCKET_OP[] = { "PUSH", "PULL" };

/** @brief A data structure that represents an exchange report
 *
 * These are sent at the end of an exchange with a farmer to report the
 * performance and reliability of farmers.
 */
typedef struct {
    char *data_hash;
    char *reporter_id;
    char *farmer_id;
    char *client_id;
    uint64_t start;
    uint64_t end;
    unsigned int code;
    char *message;
    unsigned int send_status;
    unsigned int send_count;
    uint32_t pointer_index;
} storj_exchange_report_t;

/** @brief A function signature for download/upload progress callback
 */
typedef void (*storj_progress_cb)(double progress,
                                  uint64_t bytes,
                                  uint64_t total_bytes,
                                  void *handle);

/** @brief A function signature for a download complete callback
 */
typedef void (*storj_finished_download_cb)(int status, FILE *fd, void *handle);

/** @brief A function signature for an upload complete callback
 */
typedef void (*storj_finished_upload_cb)(int error_status, storj_file_meta_t *file, void *handle);

/** @brief A structure for file upload options
 */
typedef struct {
    const char *bucket_id;
    const char *file_name;
    FILE *fd;

    /* New in V3 */
   const char *encryption_access;
   const char *content_type;
   int64_t expires;
   size_t buffer_size;

    /* NB: unused in V3 */
    const char *index;
    int prepare_frame_limit;
    int push_frame_limit;
    int push_shard_limit;
    bool rs;
} storj_upload_opts_t;

/** @brief A structure that keeps state between multiple worker threads,
 * and for referencing a download to apply actions to an in-progress download.
 *
 * After work has been completed in a thread, its after work callback will
 * update and modify the state and then queue the next set of work based on the
 * changes, and added to the event loop. The state is all managed within one
 * thread, the event loop thread, and any work that is performed in another
 * thread should not modify this structure directly, but should pass a
 * reference to it, so that once the work is complete the state can be updated.
 */
typedef struct {
    storj_env_t *env;
    DownloaderRef downloader_ref;
    const char *file_id;
    const char *bucket_id;
    storj_file_meta_t *info;
    FILE *destination;
    int error_status;
    storj_log_levels_t *log;
    void *handle;
    uint64_t total_bytes;

    storj_progress_cb progress_cb;
    storj_finished_download_cb finished_cb;
    bool finished;
    bool canceled;

    /* new in V3 */
    size_t downloaded_bytes;
    size_t buffer_size;
    const char *encryption_access;

    // TODO: delete?
    /* not used in V3 */
    bool requesting_info;
    uint32_t info_fail_count;
    uint64_t shard_size;
    uint32_t total_shards;
    int download_max_concurrency;
    uint32_t completed_shards;
    uint32_t resolving_shards;
    char *excluded_farmer_ids;
    uint32_t total_pointers;
    uint32_t total_parity_pointers;
    bool rs;
    bool recovering_shards;
    bool truncated;
    bool pointers_completed;
    uint32_t pointer_fail_count;
    bool requesting_pointers;
    bool writing;
    uint8_t *decrypt_key;
    uint8_t *decrypt_ctr;
    const char *hmac;
    uint32_t pending_work_count;
} storj_download_state_t;

typedef struct {
    storj_env_t *env;
    UploaderRef uploader_ref;
    const char *file_name;
    const char *encrypted_file_name;
    storj_file_meta_t *info;
    FILE *original_file;
    uint64_t file_size;
    const char *bucket_id;
    uint64_t uploaded_bytes;

    bool progress_finished;
    bool completed_upload;
    bool canceled;

    storj_finished_upload_cb finished_cb;
    storj_progress_cb progress_cb;
    int error_status;
    storj_log_levels_t *log;
    void *handle;

    /* new in V3 */
    size_t buffer_size;
    const char *encryption_access;
    UploadOptions *upload_opts;

    // TODO: delete?
    /* unused in V3 */
    uint8_t *encryption_key;

    uint32_t shard_concurrency;
    const char *index;
    char *bucket_key;
    uint32_t completed_shards;
    uint32_t total_shards;
    uint32_t total_data_shards;
    uint32_t total_parity_shards;
    uint64_t shard_size;
    uint64_t total_bytes;
    char *exclude;
    char *frame_id;
    char *hmac_id;
    uint8_t *encryption_ctr;

    bool rs;
    bool awaiting_parity_shards;
    char *parity_file_path;
    FILE *parity_file;
    char *encrypted_file_path;
    FILE *encrypted_file;
    bool creating_encrypted_file;

    bool requesting_frame;
    bool creating_bucket_entry;
    bool received_all_pointers;
    bool final_callback_called;
    bool bucket_verified;
    bool file_verified;

    int push_shard_limit;
    int push_frame_limit;
    int prepare_frame_limit;

    int frame_request_count;
    int add_bucket_entry_count;
    int bucket_verify_count;
    int file_verify_count;
    int create_encrypted_file_count;

    int pending_work_count;
} storj_upload_state_t;

/**
 * @brief Initialize a Storj environment
 *
 * This will setup an event loop for queueing further actions, as well
 * as define necessary configuration options for communicating with Storj
 * bridge, and for encrypting/decrypting files.
 *
 * @param[in] options - Storj Bridge API options
 * @param[in] encrypt_options - File encryption options
 * @param[in] http_options - HTTP settings
 * @param[in] log_options - Logging settings
 * @return A null value on error, otherwise a storj_env pointer.
 */
STORJ_API storj_env_t *storj_init_env(storj_bridge_options_t *bridge_options,
                                      storj_encrypt_options_t *encrypt_options,
                                      storj_http_options_t *http_options,
                                      storj_log_options_t *log_options);


/**
 * @brief Destroy a Storj environment
 *
 * This will free all memory for the Storj environment and zero out any memory
 * with sensitive information, such as passwords and encryption keys.
 *
 * The event loop must be closed before this method should be used.
 *
 * @param [in] env
 */
STORJ_API int storj_destroy_env(storj_env_t *env);

/**
 * @brief Will get the current unix timestamp in milliseconds
 *
 * @return A unix timestamp
 */
STORJ_API uint64_t storj_util_timestamp();

/**
 * @brief Get the error message for an error code
 *
 * This function will return a error message associated with a storj
 * error code.
 *
 * @param[in] error_code The storj error code integer
 * @return A char pointer with error message
 */
STORJ_API char *storj_strerror(int error_code);

/**
 * @brief Get Storj bridge API information.
 *
 * This function will get general information about the storj bridge api.
 * The network i/o is performed in a thread pool with a libuv loop, and the
 * response is available in the first argument to the callback function.
 *
 * @param[in] env The storj environment struct
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_get_info(storj_env_t *env,
                                    void *handle,
                                    uv_after_work_cb cb);

/**
 * @brief List available buckets for a user.
 *
 * @param[in] env The storj environment struct
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_get_buckets(storj_env_t *env,
                                       void *handle,
                                       uv_after_work_cb cb);

/**
 * @brief Will free all structs for get buckets request
 *
 * @param[in] req - The work request from storj_bridge_get_buckets callback
 */
STORJ_API void storj_free_get_buckets_request(get_buckets_request_t *req);

/**
 * @brief Create a bucket.
 *
 * @param[in] env The storj environment struct
 * @param[in] name The name of the bucket
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_create_bucket(storj_env_t *env,
                                         const char *name,
                                         BucketConfig *cfg,
                                         void *handle,
                                         uv_after_work_cb cb);

/**
 * @brief Delete a bucket.
 *
 * @param[in] env The storj environment struct
 * @param[in] id The bucket id
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_delete_bucket(storj_env_t *env,
                                         const char *id,
                                         void *handle,
                                         uv_after_work_cb cb);

/**
 * @brief Get a info of specific bucket.
 *
 * @param[in] env The storj environment struct
 * @param[in] id The bucket id
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_get_bucket(storj_env_t *env,
                                      const char *id,
                                      void *handle,
                                      uv_after_work_cb cb);

/**
 * @brief Will free all structs for get bucket request
 *
 * @param[in] req - The work request from storj_bridge_get_bucket callback
 */
STORJ_API void storj_free_get_bucket_request(get_bucket_request_t *req);

/**
 * @brief Will free all structs for create bucket request
 *
 * @param[in] req - The work request from storj_bridge_create_bucket callback
 */
STORJ_API void storj_free_create_bucket_request(create_bucket_request_t *req);

/**
 * @brief Get the bucket id by name.
 *
 * @param[in] env The storj environment struct
 * @param[in] name The bucket name
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_get_bucket_id(storj_env_t *env,
                                         const char *name,
                                         void *handle,
                                         uv_after_work_cb cb);

/**
 * @brief Get a list of all files in a bucket.
 *
 * @param[in] env The storj environment struct
 * @param[in] id The bucket id
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_list_files(storj_env_t *env,
                                      const char *id,
                                      const char *encryption_access,
                                      void *handle,
                                      uv_after_work_cb cb);

/**
 * @brief Will free all pointers for file_meta struct.
 *
 * @param[in] file_meta struct to free.
 */
STORJ_API void storj_free_file_meta(storj_file_meta_t *file_meta);

/**
 * @brief Will free all structs for list files request
 *
 * @param[in] req - The work request from storj_bridge_list_files callback
 */
STORJ_API void storj_free_list_files_request(list_files_request_t *req);

/**
 * @brief Create a PUSH or PULL bucket token.
 *
 * @param[in] env The storj environment struct
 * @param[in] bucket_id The bucket id
 * @param[in] operation The type of operation PUSH or PULL
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_create_bucket_token(storj_env_t *env,
                                               const char *bucket_id,
                                               storj_bucket_op_t operation,
                                               void *handle,
                                               uv_after_work_cb cb);

/**
 * @brief Get pointers with locations to file shards.
 *
 * @param[in] env The storj environment struct
 * @param[in] bucket_id The bucket id
 * @param[in] file_id The bucket id
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_get_file_pointers(storj_env_t *env,
                                             const char *bucket_id,
                                             const char *file_id,
                                             void *handle,
                                             uv_after_work_cb cb);

/**
 * @brief Delete a file in a bucket.
 *
 * @param[in] env The storj environment struct
 * @param[in] bucket_id The bucket id
 * @param[in] file_id The bucket id
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_delete_file(storj_env_t *env,
                                       const char *bucket_id,
                                       const char *file_id,
                                       const char *encryption_access,
                                       void *handle,
                                       uv_after_work_cb cb);

/**
 * @brief Create a file frame
 *
 * @param[in] env The storj environment struct
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_create_frame(storj_env_t *env,
                                        void *handle,
                                        uv_after_work_cb cb);

/**
 * @brief List available file frames
 *
 * @param[in] env The storj environment struct
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_get_frames(storj_env_t *env,
                                      void *handle,
                                      uv_after_work_cb cb);

/**
 * @brief Get information for a file frame
 *
 * @param[in] env The storj environment struct
 * @param[in] frame_id The frame id
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
 STORJ_API int storj_bridge_get_frame(storj_env_t *env,
                                      const char *frame_id,
                                      void *handle,
                                      uv_after_work_cb cb);

/**
 * @brief Delete a file frame
 *
 * @param[in] env The storj environment struct
 * @param[in] frame_id The frame id
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_delete_frame(storj_env_t *env,
                                        const char *frame_id,
                                        void *handle,
                                        uv_after_work_cb cb);

/**
 * @brief Get metadata for a file
 *
 * @param[in] env The storj environment struct
 * @param[in] bucket_id The bucket id
 * @param[in] file_id The file id
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_get_file_info(storj_env_t *env,
                                         const char *bucket_id,
                                         const char *file_id,
                                         const char *encryption_access,
                                         void *handle,
                                         uv_after_work_cb cb);

/**
 * @brief Will free all structs for get file info request
 *
 * @param[in] req - The work request from storj_bridge_get_file_info callback
 */
STORJ_API void storj_free_get_file_info_request(get_file_info_request_t *req);

/**
 * @brief Will free all structs for delete file request
 *
 * @param[in] req - The work request from storj_bridge_delete_file callback
 */
STORJ_API void storj_free_delete_file_request(delete_file_request_t *req);

/**
 * @brief Get the file id by name.
 *
 * @param[in] env The storj environment struct
 * @param[in] bucket_id The bucket id
 * @param[in] file_name The file name
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_get_file_id(storj_env_t *env,
                                       const char *bucket_id,
                                       const char *file_name,
                                       void *handle,
                                       uv_after_work_cb cb);

/**
 * @brief Get mirror data for a file
 *
 * @param[in] env The storj environment struct
 * @param[in] bucket_id The bucket id
 * @param[in] file_id The bucket id
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] cb A function called with response when complete
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_list_mirrors(storj_env_t *env,
                                        const char *bucket_id,
                                        const char *file_id,
                                        void *handle,
                                        uv_after_work_cb cb);

/**
 * @brief Will cancel an upload
 *
 * @param[in] state A pointer to the the upload state
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_store_file_cancel(storj_upload_state_t *state);

/**
 * @brief Upload a file
 *
 * @param[in] env A pointer to environment
 * @param[in] state A pointer to the the upload state
 * @param[in] opts The options for the upload
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] progress_cb Function called with progress updates
 * @param[in] finished_cb Function called when download finished
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API storj_upload_state_t *storj_bridge_store_file(storj_env_t *env,
                                                        storj_upload_opts_t *opts,
                                                        void *handle,
                                                        storj_progress_cb progress_cb,
                                                        storj_finished_upload_cb finished_cb);

/**
 * @brief Will free the file info struct passed to the upload finished callback
 *
 * @param[in] file - The storj_file_meta_t struct from storj_finished_upload_cb callback
 */
STORJ_API void storj_free_uploaded_file_info(storj_file_meta_t *file);

/**
 * @brief Will cancel a download
 *
 * @param[in] state A pointer to the the download state
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API int storj_bridge_resolve_file_cancel(storj_download_state_t *state);

/**
 * @brief Download a file
 *
 * @param[in] env A pointer to environment
 * @param[in] state A pointer to the the download state
 * @param[in] bucket_id Character array of bucket id
 * @param[in] file_id Character array of file id
 * @param[in] destination File descriptor of the destination
 * @param[in] handle A pointer that will be available in the callback
 * @param[in] progress_cb Function called with progress updates
 * @param[in] finished_cb Function called when download finished
 * @return A non-zero error value on failure and 0 on success.
 */
STORJ_API storj_download_state_t *storj_bridge_resolve_file(storj_env_t *env,
                                                            const char *bucket_id,
                                                            const char *file_id,
                                                            FILE *destination,
                                                            const char *encryption_access,
                                                            size_t buffer_size,
                                                            void *handle,
                                                            storj_progress_cb progress_cb,
                                                            storj_finished_download_cb finished_cb);

#ifdef __cplusplus
}
#endif

#endif /* STORJ_H */
