/**
 * @file storjapi_callback.h
 * @brief Storj callback library.
 *
 * Implements callback functionality that can be customised for 
 * end user's application
 */

#ifndef CLI_CALLBACK_H
#define CLI_CALLBACK_H

#ifdef __cplusplus
extern "C" {
#endif

#include "storj.h"

#define CLI_NO_SUCH_FILE_OR_DIR   0x00
#define CLI_VALID_REGULAR_FILE    0x01
#define CLI_VALID_DIR             0x02
#define CLI_UNKNOWN_FILE_ATTR     0x03
#define CLI_UPLOAD_FILE_LOG_ERR   0x04

/**
 * @brief A Structure for passing the User's Application info to
 *        Storj API.
 */
typedef struct cli_api {
    storj_env_t *env;
    storj_file_meta_t *files;
    char *bucket_name;
    char bucket_id[256];
    char *file_name;
    char file_id[256];
    char *file_path;     /**< local upload files directory path */
    FILE *src_fd;
    char src_list[256];      /**< file list ready to upload */
    char *src_file;      /**< next file ready to upload */
    FILE *dst_fd;
    char *dst_file;      /**< next file ready to upload */
    int  xfer_count;     /**< # of files xferred (up/down) */
    int  total_files;    /**< total files to upload */
    char *last_cmd_req;  /**< last command requested */
    char *curr_cmd_req;  /**< cli curr command requested */
    char *next_cmd_req;  /**< cli curr command requested */
    char *final_cmd_req; /**< final command in the seq */
    char *excp_cmd_resp; /**< expected cmd response */
    char *rcvd_cmd_resp; /**< received cmd response */
    int  error_status;   /**< command response/error status */
    storj_log_levels_t *log;
    void *handle;
} cli_api_t;

/**
 * @brief Callback function listing bucket names & IDs 
 */
void get_buckets_callback(uv_work_t *work_req, int status);

/**
 * @brief Callback function returning the bucket id for a given
 *        bucket name
 */
void get_bucket_id_callback(uv_work_t *work_req, int status);

/**
 * @brief Callback function returning the file id for a given
 *        file name
 */
void get_file_id_callback(uv_work_t *work_req, int status);

/**
 * @brief Storj api state machine function 
 */
void queue_next_cmd_req(cli_api_t *cli_api);

/**
 * @brief Function lists the bucket names & IDs 
 * 
 * @param[in] cli_api_t structure that passes user's input
 *       info
 * @return A non-zero error value on failure and 0 on success.
 */
int cli_list_buckets(cli_api_t *cli_api);

/**
 * @brief Function returns the corresponding bucket's id for a 
 *        given bucket name
 * 
 * @param[in] cli_api_t structure that passes user's input
 *       info
 * @return A non-zero error value on failure and 0 on success.
 */
int cli_get_bucket_id(cli_api_t *cli_api);

/**
 * @brief Function to list files in a given bucket name
 * 
 * @param[in] cli_api_t structure that passes user's input
 *       info
 * @return A non-zero error value on failure and 0 on success.
 */
int cli_list_files(cli_api_t *cli_api);

/**
 * @brief Function to remove a given bucket name 
 * 
 * @param[in] cli_api_t structure that passes user's input
 *       info
 * @return A non-zero error value on failure and 0 on success.
 */
int cli_remove_bucket(cli_api_t *cli_api);

/**
 * @brief Function to remove a file from a given bucket name 
 * 
 * @param[in] cli_api_t structure that passes user's input
 *       info
 * @return A non-zero error value on failure and 0 on success.
 */
int cli_remove_file(cli_api_t *cli_api);

/**
 * @brief Function to return the node IDs for a given file for a
 *        given bucket name
 * 
 * @param[in] cli_api_t structure that passes user's input
 *       info
 * @return A non-zero error value on failure and 0 on success.
 */
int cli_list_mirrors(cli_api_t *cli_api);

/**
 * @brief Function to upload a local file into a given bucket 
 *        name
 * 
 * @param[in] cli_api_t structure that passes user's input
 *       info
 * @return A non-zero error value on failure and 0 on success.
 */
int cli_upload_file(cli_api_t *cli_api);

/**
 * @brief Function to upload local files into a given bucket 
 *        name
 * 
 * @param[in] cli_api_t structure that passes user's input
 *       info
 * @return A non-zero error value on failure and 0 on success.
 */
int cli_upload_files(cli_api_t *cli_api);

/**
 * @brief Function to download a file from a given bucket to a 
 *        local folder
 * 
 * @param[in] cli_api_t structure that passes user's input
 *       info
 * @return A non-zero error value on failure and 0 on success.
 */
int cli_download_file(cli_api_t *cli_api);

/**
 * @brief Function to download files from a given bucket to a 
 *        local folder
 * 
 * @param[in] cli_api_t structure that passes user's input
 *       info
 * @return A non-zero error value on failure and 0 on success.
 */
int cli_download_files(cli_api_t *cli_api);

#ifdef __cplusplus
}
#endif

#endif /* CLI_CALLBACK_H */
