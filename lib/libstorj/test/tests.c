#include "storjtests.h"

char *folder;
int tests_ran = 0;
int test_status = 0;
const char *test_bucket_name = "test-bucket";
const char *test_upload_file_name = "test-upload-file";
const char *test_download_file_name = "test-download-file";
char *test_download_path;
char *test_upload_path;

double test_upload_progress = 0;
uint64_t test_uploaded_bytes = 0;
uint64_t test_total_bytes = 0;

BucketConfig test_bucket_cfg = {
    .path_cipher = STORJ_ENC_AESGCM,

    .encryption_parameters.cipher_suite = STORJ_ENC_AESGCM,
    .encryption_parameters.block_size = 2048,

    .redundancy_scheme.algorithm = STORJ_REED_SOLOMON,
    .redundancy_scheme.share_size = 1024,
    .redundancy_scheme.required_shares = 2,
    .redundancy_scheme.repair_shares = 4,
    .redundancy_scheme.optimal_shares = 5,
    .redundancy_scheme.total_shares = 6
};

storj_bridge_options_t bridge_options;
storj_encrypt_options_t encrypt_options = {
    .key = { 0x31, 0x32, 0x33, 0x61, 0x33, 0x32, 0x31 }
};

storj_upload_opts_t upload_options = {
    // NB: about +500 years from time of writing
    .expires = 17329017831
};

storj_log_options_t log_options = {
    .level = 4
};

char *test_encryption_access;

void fail(char *msg)
{
    printf("\t" KRED "FAIL" RESET " %s\n", msg);
    tests_ran += 1;
}

void pass(char *msg)
{
    printf("\t" KGRN "PASS" RESET " %s\n", msg);
    test_status += 1;
    tests_ran += 1;
}

void check_get_buckets(uv_work_t *work_req, int status)
{
    require_no_last_error_if(status);

    // TODO: require req->error_code & req->status_code
    // (status_code is an http status)

    get_buckets_request_t *req = work_req->data;

    // TODO: add assertions
    require(req->buckets);
    require(req->total_buckets == 1);

    pass("storj_bridge_get_buckets");

    storj_free_get_buckets_request(req);
    free(work_req);
}

void check_get_bucket(uv_work_t *work_req, int status)
{
    require_no_last_error_if(status);

    // TODO: require req->error_code & req->status_code
    // (status_code is an http status)

    get_bucket_request_t *req = work_req->data;

    require(!req->handle);
    require(req->bucket);
    require(req->bucket->decrypted);

    require_equal(test_bucket_name, req->bucket->name);
    require_equal(test_bucket_name, req->bucket->id);

    pass("storj_bridge_get_bucket");

    storj_free_get_bucket_request(req);
    free(work_req);
}

void check_get_bucket_id(uv_work_t *work_req, int status)
{
    require_no_last_error_if(status);

    get_bucket_id_request_t *req = work_req->data;

    require(!req->handle);

    require_equal(test_bucket_name, req->bucket_id);

    pass("storj_bridge_get_bucket_id");

    json_object_put(req->response);
    free((char *)req->bucket_name);
    free((char *)req->bucket_id);
    free(req);
    free(work_req);
}

void check_create_bucket(uv_work_t *work_req, int status)
{
    require_no_last_error;

    // TODO: require req->error_code & req->status_code
    // (status_code is an http status)

    require(status == 0);
    create_bucket_request_t *req = work_req->data;

    require(req->bucket);

    require_not_empty(req->bucket->created);

    require_equal(test_bucket_name, req->bucket_name);
    require_equal(test_bucket_name, req->bucket->name);
    require_equal(test_bucket_name, req->bucket->id);

    pass("storj_bridge_create_bucket");

    storj_free_create_bucket_request(req);
    free(work_req);
}

void check_list_files(uv_work_t *work_req, int status)
{
    require_no_last_error;

    // TODO: maybe should be `require(req->status_code == 0);`?
    require(status == 0);
    list_files_request_t *req = work_req->data;
    require(!req->handle);
    require(!req->response);
    require(req->total_files == 1);

    require_equal(test_bucket_name, req->bucket_id);

    // TODO: add assertions?

    pass("storj_bridge_list_files");

    storj_free_list_files_request(req);
    free(work_req);
}

void check_delete_bucket(uv_work_t *work_req, int status)
{
    require_no_last_error;

    require(status == 0);
    delete_bucket_request_t *req = work_req->data;
    require(!req->handle);
    require(!req->response);
    require(req->status_code == 204);

    // TODO: check that the bucket was actuallly deleted!

    pass("storj_bridge_delete_bucket");

    free((char *)req->bucket_name);
    free(req);
    free(work_req);
}

void check_get_file_id(uv_work_t *work_req, int status)
{
    require_no_last_error_if(status);

    get_file_id_request_t *req = work_req->data;
    require(!req->handle);
    require_equal(test_upload_file_name, req->file_id);

    pass("storj_bridge_get_file_id");

    json_object_put(req->response);
    free(req);
    free(work_req);
}

void check_resolve_file_progress(double progress,
                                 uint64_t downloaded_bytes,
                                 uint64_t total_bytes,
                                 void *handle)
{
    require(handle == NULL);
    if (progress == (double)1) {
        pass("storj_bridge_resolve_file (progress finished)");
    }

    // TODO check error case
}

void check_resolve_file(int status, FILE *fd, void *handle)
{
    fclose(fd);
    require(handle == NULL);
    if (status) {
        fail("storj_bridge_resolve_file");
        printf("Download failed: %s\n", storj_strerror(status));
    } else {
        pass("storj_bridge_resolve_file");
    }
}

void check_resolve_file_cancel(int status, FILE *fd, void *handle)
{
    fclose(fd);
    require(handle == NULL);
    if (status == STORJ_TRANSFER_CANCELED) {
        pass("storj_bridge_resolve_file_cancel");
    } else {
        fail("storj_bridge_resolve_file_cancel");
    }
}

void check_store_file_progress(double progress,
                               uint64_t uploaded_bytes,
                               uint64_t total_bytes,
                               void *handle)
{
    require_no_last_error;

    require(progress >= test_upload_progress);
    require(uploaded_bytes >= test_uploaded_bytes);

    if (test_total_bytes == 0) {
        test_total_bytes = total_bytes;
    }

    require(total_bytes == test_total_bytes);

    test_upload_progress = progress;
    test_uploaded_bytes = uploaded_bytes;

    require(handle == NULL);
    if (progress == (double)0) {
        pass("storj_bridge_store_file (progress started)");
    }
    if (progress == (double)1) {
        pass("storj_bridge_store_file (progress finished)");
    }
}

void check_store_file_progress_cancel(double progress,
                               uint64_t uploaded_bytes,
                               uint64_t total_bytes,
                               void *handle)
{
    require_no_last_error;

    require(!(progress > test_upload_progress));
    require(!(uploaded_bytes > test_uploaded_bytes));

    test_upload_progress = progress;
    test_uploaded_bytes = uploaded_bytes;

    require(handle == NULL);
    if (progress != (double)1) {
        pass("storj_bridge_store_file (progress incomplete)");
    }
}

void check_store_file(int error_code, storj_file_meta_t *info, void *handle)
{
    require_no_last_error;

    require(!handle);
    require(info);

    require_not_empty(info->id);
    require_not_empty(info->bucket_id);
    require_not_empty(info->created);
    require_not_empty(info->mimetype);

    require_equal(test_upload_file_name, info->id);
    require_equal(test_bucket_name, info->bucket_id);

    // TODO: more assertions?

    pass("storj_bridge_store_file");

    storj_free_uploaded_file_info(info);
}

void check_store_file_cancel(int error_code, storj_file_meta_t *file, void *handle)
{
    require(handle == NULL);
    if (error_code == STORJ_TRANSFER_CANCELED) {
        pass("storj_bridge_store_file_cancel");
    } else {
        fail("storj_bridge_store_file_cancel");
        printf("\t\tERROR:   %s\n", storj_strerror(error_code));
    }

    storj_free_uploaded_file_info(file);
}

//void check_delete_file(uv_work_t *work_req, int status)
//{
//    require(status == 0);
//    json_request_t *req = work_req->data;
//    require(req->handle == NULL);
//    require(req->response == NULL);
//    require(req->status_code == 200);
//
//    pass("storj_bridge_delete_file");
//
//    free(req->path);
//    free(req);
//    free(work_req);
//}

void check_file_info(uv_work_t *work_req, int status)
{
    require_no_last_error;

    require(status == 0);
    get_file_info_request_t *req = work_req->data;
    require(!req->handle);
    require(req->file);
    // TODO: more precise size assertion
//    require(req->file->size > 0);

    require_not_empty(req->file->created);
    require_not_empty(req->file->mimetype);

    require_equal(test_upload_file_name, req->file->id);
    require_equal(test_upload_file_name, req->file->filename);
    require_equal(test_bucket_name, req->file->bucket_id);

    // TODO: add assertions?

    pass("storj_bridge_get_file_info");

    storj_free_get_file_info_request(req);
    free(work_req);
}

int create_test_upload_file(char *filepath)
{
    FILE *fp;
    fp = fopen(filepath, "w");

    if (fp == NULL) {
        printf(KRED "Could not create upload file: %s\n" RESET, filepath);
        exit(0);
    }

    int shard_size = 4096;
    char *bytes = "abcdefghijklmn";
    for (int i = 0; i < strlen(bytes); i++) {
        char *page = calloc(shard_size + 1, sizeof(char));
        memset(page, bytes[i], shard_size);
        fputs(page, fp);
        free(page);
    }

    fclose(fp);
    return 0;
}

int test_upload(storj_env_t *env)
{
    // upload file
    storj_upload_state_t *state = storj_bridge_store_file(env,
                                                          &upload_options,
                                                          NULL,
                                                          check_store_file_progress,
                                                          check_store_file);
    require(state != NULL);
    require_no_last_error_if(state->error_status);

    // run all queued events
    require_no_last_error_if(uv_run(env->loop, UV_RUN_DEFAULT));
    return 0;
}

int test_upload_cancel(storj_env_t *env)
{
    create_test_upload_file(strdup(test_upload_path));

    // upload file
    storj_upload_state_t *state = storj_bridge_store_file(env,
                                                          &upload_options,
                                                          NULL,
                                                          check_store_file_progress_cancel,
                                                          check_store_file_cancel);
    require(state != NULL);
    require_no_last_error_if(state->error_status);

    // run all queued events
//    require_no_last_error_if(uv_run(env->loop, UV_RUN_DEFAULT));
//    uv_run(env->loop, UV_RUN_ONCE);

    storj_bridge_store_file_cancel(state);
    require_no_last_error_if(uv_run(env->loop, UV_RUN_DEFAULT));

    // TODO: needs to go before `uv_run`?
//    storj_bridge_store_file_cancel(state);

    return 0;
}

int test_download(storj_env_t *env)
{
    // resolve file
    FILE *file = fopen(test_download_path, "w+");

    storj_download_state_t *state = storj_bridge_resolve_file(env,
                                                              test_bucket_name,
                                                              test_upload_file_name,
                                                              file,
                                                              test_encryption_access,
                                                              NULL,
                                                              check_resolve_file_progress,
                                                              check_resolve_file);

    if (!state || state->error_status != 0) {
        return 1;
    }

    if (uv_run(env->loop, UV_RUN_DEFAULT)) {
        return 1;
    }

    return 0;
}

//int test_download_cancel()
//{
//
//    // initialize event loop and environment
//    storj_env_t *env = storj_init_env(&bridge_options,
//                                      &encrypt_options,
//                                      &http_options,
//                                      &log_options);
//    require(env != NULL);
//
//    // resolve file
//    char *download_file = calloc(strlen(folder) + 33 + 1, sizeof(char));
//    strcpy(download_file, folder);
//    strcat(download_file, "storj-test-download-canceled.data");
//    FILE *download_fp = fopen(download_file, "w+");
//
//    char *bucket_id = "368be0816766b28fd5f43af5";
//    char *file_id = "998960317b6725a3f8080c2b";
//
//    storj_download_state_t *state = storj_bridge_resolve_file(env,
//                                                              bucket_id,
//                                                              file_id,
//                                                              download_fp,
//                                                              NULL,
//                                                              check_resolve_file_progress,
//                                                              check_resolve_file_cancel);
//
//    if (!state || state->error_status != 0) {
//        return 1;
//    }
//
//    // process the loop one at a time so that we can do other things while
//    // the loop is processing, such as cancel the download
//    int count = 0;
//    bool more;
//    int status = 0;
//    do {
//        more = uv_run(env->loop, UV_RUN_ONCE);
//        if (more == false) {
//            more = uv_loop_alive(env->loop);
//            if (uv_run(env->loop, UV_RUN_NOWAIT) != 0) {
//                more = true;
//            }
//        }
//
//        count++;
//
//        if (count == 100) {
//            status = storj_bridge_resolve_file_cancel(state);
//            require(status == 0);
//        }
//
//    } while (more == true);
//
//
//    free(download_file);
//    storj_destroy_env(env);
//
//    return 0;
//}

static void reset_test_upload()
{
    test_upload_progress = 0;
    test_uploaded_bytes = 0;
    test_total_bytes = 0;

    // init upload options
    upload_options.bucket_id = strdup(test_bucket_name);
    upload_options.file_name = strdup(test_upload_file_name);
    upload_options.fd = fopen(test_upload_path, "r");
    upload_options.encryption_access = strdup(test_encryption_access);
}

int test_api()
{
    // initialize environment
    storj_env_t *env = storj_init_env(&bridge_options,
                                      &encrypt_options,
                                      NULL,
                                      &log_options);
    require_no_last_error;
    require(env != NULL);

    int status;

    // create bucket
    status = storj_bridge_create_bucket(env, test_bucket_name, &test_bucket_cfg,
                                        NULL, check_create_bucket);
    require_no_last_error_if(status);
    require_no_last_error_if(uv_run(env->loop, UV_RUN_ONCE));

    // list buckets
    status = storj_bridge_get_buckets(env, NULL, check_get_buckets);
    require_no_last_error_if(status);
    require_no_last_error_if(uv_run(env->loop, UV_RUN_ONCE));

    // get bucket
    status = storj_bridge_get_bucket(env, test_bucket_name, NULL, check_get_bucket);
    require_no_last_error_if(status);
    require_no_last_error_if(uv_run(env->loop, UV_RUN_ONCE));

    // get bucket id
    // NB: bucket id isn't a thing anymore; replacing id with the name.
    //      Additionally, buckets are always decrypted.
    status = storj_bridge_get_bucket_id(env, test_bucket_name, NULL, check_get_bucket_id);
    require_no_last_error_if(status);
    require_no_last_error_if(uv_run(env->loop, UV_RUN_ONCE));

    // upload file
    reset_test_upload();
    test_upload(env);
    require_no_last_error;

    reset_test_upload();
    test_upload_cancel(env);
    require_no_last_error;

//    test_download(env);
    // TODO: download cancel

    // list files
    status = storj_bridge_list_files(env, test_bucket_name,
                                     test_encryption_access,
                                     NULL, check_list_files);
    require_no_last_error_if(status);
    require_no_last_error_if(uv_run(env->loop, UV_RUN_ONCE));

    // get file id
    // NB: file id isn't a thing anymore; replacing id with the name.
    status = storj_bridge_get_file_id(env, test_bucket_name, test_upload_file_name,
                                      NULL, check_get_file_id);
    require_no_last_error_if(status);
    require_no_last_error_if(uv_run(env->loop, UV_RUN_ONCE));

    // get file info
    status = storj_bridge_get_file_info(env, test_bucket_name,test_upload_file_name,
                                        test_encryption_access, NULL,
                                        check_file_info);
    require_no_last_error_if(status);
    require_no_last_error_if(uv_run(env->loop, UV_RUN_ONCE));


//    // delete a file in a bucket
//    status = storj_bridge_delete_file(env,
//                                      bucket_id,
//                                      file_id,
//                                      NULL,
//                                      check_delete_file);
//    require(status == 0);
//

    // delete bucket
    status = storj_bridge_delete_bucket(env, test_bucket_name,
                                        NULL, check_delete_bucket);
    require_no_last_error_if(status);
    require_no_last_error_if(uv_run(env->loop, UV_RUN_ONCE));

    storj_destroy_env(env);
    return 0;
}

int main(void)
{
    // setup bridge options to point to testplanet
    bridge_options.addr = getenv("SATELLITE_0_ADDR");
    bridge_options.apikey = getenv("GATEWAY_0_API_KEY");

    EncryptionAccessRef encryption_access = new_encryption_access_with_default_key(bridge_options.apikey);
    test_encryption_access = serialize_encryption_access(encryption_access, STORJ_LAST_ERROR);
    require_no_last_error;
    require(test_encryption_access && strcmp("", test_encryption_access) != 0);

    // Make sure we have a tmp folder
    folder = getenv("TMPDIR");

    if (folder == 0) {
        printf("You need to set $TMPDIR before running. (e.g. export TMPDIR=/tmp/)\n");
        exit(1);
    }

    // Set test file name
    int upload_name_len = 1 + strlen(folder) + strlen(test_upload_file_name);
    int download_name_len = 1 + strlen(folder) + strlen(test_download_file_name);
    test_upload_path = calloc(upload_name_len , sizeof(char));
    test_download_path = calloc(download_name_len , sizeof(char));
    strcpy(test_upload_path, folder);
    strcpy(test_download_path, folder);
    #ifdef _WIN32
        strcat(test_upload_path, "\\");
        strcat(test_download_path, "\\");
    #else
        strcat(test_upload_path, "/");
        strcat(test_download_path, "/");
    #endif
    strcat(test_upload_path, test_upload_file_name);
    strcat(test_download_path, test_download_file_name);

    create_test_upload_file(strdup(test_upload_path));

    printf("Test Suite: API\n");
    test_api();

    free(test_upload_path);
    free(test_download_path);

    int num_failed = tests_ran - test_status;
    printf(KGRN "\nPASSED: %i" RESET, test_status);
    if (num_failed > 0) {
        printf(KRED " FAILED: %i" RESET, num_failed);
    }
    printf(" TOTAL: %i\n", (tests_ran));

    if (num_failed > 0) {
        return 1;
    }

    return 0;
}
