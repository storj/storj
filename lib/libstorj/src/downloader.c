#include "downloader.h"

static void free_download_state(storj_download_state_t *state)
{
//    for (int i = 0; i < state->total_pointers; i++) {
//        storj_pointer_t *pointer = &state->pointers[i];
//
//        free(pointer->token);
//        free(pointer->shard_hash);
//        free(pointer->farmer_id);
//        free(pointer->farmer_address);
//
//        free_exchange_report(pointer->report);
//    }
//
//    if (state->excluded_farmer_ids) {
//        free(state->excluded_farmer_ids);
//    }
//
//    if (state->decrypt_key) {
//        memset_zero(state->decrypt_key, SHA256_DIGEST_SIZE);
//        free(state->decrypt_key);
//    }
//
//    if (state->decrypt_ctr) {
//        memset_zero(state->decrypt_ctr, AES_BLOCK_SIZE);
//        free(state->decrypt_ctr);
//    }
//
//    if (state->info) {
//        if (state->info->erasure) {
//            free((char *)state->info->erasure);
//        }
//        free((char *)state->info->hmac);
//        free(state->info);
//    }
//
//    if (state->hmac)  {
//        free((char *)state->hmac);
//    }
//
//    free(state->pointers);
//    free(state);
}

//static void after_request_info(uv_work_t *work, int status)
//{
//    file_info_request_t *req = work->data;
//
//    req->state->pending_work_count--;
//    req->state->requesting_info = false;
//
//    if (status != 0) {
//        req->state->error_status = STORJ_BRIDGE_FILEINFO_ERROR;
//    } else if (req->status_code == 200 || req->status_code == 304) {
//        req->state->info = req->info;
//        if (req->info->erasure) {
//            if (strcmp(req->info->erasure, "reedsolomon") == 0) {
//                req->state->rs = true;
//                req->state->truncated = false;
//            } else {
//                req->state->error_status = STORJ_FILE_UNSUPPORTED_ERASURE;
//            }
//        }
//
//        // Now that we have info we can calculate the decryption key
//        determine_decryption_key(req->state);
//
//    } else if (req->error_status) {
//        switch(req->error_status) {
//            case STORJ_BRIDGE_REQUEST_ERROR:
//            case STORJ_BRIDGE_INTERNAL_ERROR:
//                req->state->info_fail_count += 1;
//                break;
//            default:
//                req->state->error_status = req->error_status;
//                break;
//        }
//        if (req->state->info_fail_count >= STORJ_MAX_INFO_TRIES) {
//            req->state->info_fail_count = 0;
//            req->state->error_status = req->error_status;
//        }
//    } else {
//        req->state->error_status = STORJ_BRIDGE_FILEINFO_ERROR;
//    }
//
//    queue_next_work(req->state);
//
//    free(req);
//    free(work);
//
//}
//
//static void request_info(uv_work_t *work)
//{
//    file_info_request_t *req = work->data;
//    storj_download_state_t *state = req->state;
//
//    int path_len = 9 + strlen(req->bucket_id) + 7 + strlen(req->file_id) + 5;
//    char *path = calloc(path_len + 1, sizeof(char));
//    if (!path) {
//        req->error_status = STORJ_MEMORY_ERROR;
//        return;
//    }
//
//    strcat(path, "/buckets/");
//    strcat(path, req->bucket_id);
//    strcat(path, "/files/");
//    strcat(path, req->file_id);
//    strcat(path, "/info");
//
//    int status_code = 0;
//    struct json_object *response = NULL;
//    int request_status = fetch_json(req->http_options,
//                                    req->options,
//                                    "GET",
//                                    path,
//                                    NULL,
//                                    true,
//                                    &response,
//                                    &status_code);
//
//    req->status_code = status_code;
//
//    state->log->debug(state->env->log_options,
//                      state->handle,
//                      "fn[request_info] - JSON Response: %s",
//                      json_object_to_json_string(response));
//
//    if (request_status) {
//        req->error_status = STORJ_BRIDGE_REQUEST_ERROR;
//        state->log->warn(state->env->log_options, state->handle,
//                         "Request file info error: %i", request_status);
//
//    } else if (status_code == 200 || status_code == 304) {
//
//        req->info = malloc(sizeof(storj_file_meta_t));
//        req->info->created = NULL;
//        req->info->filename = NULL;
//        req->info->mimetype = NULL;
//        req->info->erasure = NULL;
//        req->info->size = 0;
//        req->info->hmac = NULL;
//        req->info->id = NULL;
//        req->info->bucket_id = NULL;
//        req->info->decrypted = false;
//        req->info->index = NULL;
//
//        struct json_object *erasure_obj;
//        struct json_object *erasure_value;
//        char *erasure = NULL;
//        if (json_object_object_get_ex(response, "erasure", &erasure_obj)) {
//            if (json_object_object_get_ex(erasure_obj, "type", &erasure_value)) {
//                erasure = (char *)json_object_get_string(erasure_value);
//            }   else {
//                state->log->warn(state->env->log_options, state->handle,
//                                 "value missing from erasure response");
//            }
//        }
//
//        if (erasure) {
//            req->info->erasure = strdup(erasure);
//        }
//
//        struct json_object *index_value;
//        char *index = NULL;
//        if (json_object_object_get_ex(response, "index", &index_value)) {
//            index = (char *)json_object_get_string(index_value);
//        }
//
//        if (index) {
//            req->info->index = strdup(index);
//        }
//
//        struct json_object *hmac_obj;
//        if (!json_object_object_get_ex(response, "hmac", &hmac_obj)) {
//            state->log->warn(state->env->log_options, state->handle,
//                             "hmac missing from response");
//            goto clean_up;
//        }
//        if (!json_object_is_type(hmac_obj, json_type_object)) {
//            state->log->warn(state->env->log_options, state->handle,
//                             "hmac not an object");
//            goto clean_up;
//        }
//
//        // check the type of hmac
//        struct json_object *hmac_type;
//        if (!json_object_object_get_ex(hmac_obj, "type", &hmac_type)) {
//            state->log->warn(state->env->log_options, state->handle,
//                             "hmac.type missing from response");
//            goto clean_up;
//        }
//        if (!json_object_is_type(hmac_type, json_type_string)) {
//            state->log->warn(state->env->log_options, state->handle,
//                             "hmac.type not a string");
//            goto clean_up;
//        }
//        char *hmac_type_str = (char *)json_object_get_string(hmac_type);
//        if (0 != strcmp(hmac_type_str, "sha512")) {
//            state->log->warn(state->env->log_options, state->handle,
//                             "hmac.type is unknown");
//            goto clean_up;
//        }
//
//        // get the hmac value
//        struct json_object *hmac_value;
//        if (!json_object_object_get_ex(hmac_obj, "value", &hmac_value)) {
//            state->log->warn(state->env->log_options, state->handle,
//                             "hmac.value missing from response");
//            goto clean_up;
//        }
//        if (!json_object_is_type(hmac_value, json_type_string)) {
//            state->log->warn(state->env->log_options, state->handle,
//                             "hmac.value not a string");
//            goto clean_up;
//        }
//        char *hmac = (char *)json_object_get_string(hmac_value);
//        req->info->hmac = strdup(hmac);
//
//    } else if (status_code == 403 || status_code == 401) {
//        req->error_status = STORJ_BRIDGE_AUTH_ERROR;
//    } else if (status_code == 404 || status_code == 400) {
//        req->error_status = STORJ_BRIDGE_FILE_NOTFOUND_ERROR;
//    } else if (status_code == 500) {
//        req->error_status = STORJ_BRIDGE_INTERNAL_ERROR;
//    } else {
//        req->error_status = STORJ_BRIDGE_REQUEST_ERROR;
//    }
//
//clean_up:
//    if (response) {
//        json_object_put(response);
//    }
//    free(path);
//}
//
//static void queue_request_info(storj_download_state_t *state)
//{
//    if (state->requesting_info || state->canceled) {
//        return;
//    }
//
//    uv_work_t *work = malloc(sizeof(uv_work_t));
//    if (!work) {
//        state->error_status = STORJ_MEMORY_ERROR;
//        return;
//    }
//
//    state->requesting_info = true;
//
//    file_info_request_t *req = malloc(sizeof(file_info_request_t));
//    req->http_options = state->env->http_options;
//    req->options = state->env->bridge_options;
//    req->status_code = 0;
//    req->bucket_id = state->bucket_id;
//    req->file_id = state->file_id;
//    req->error_status = 0;
//    req->info = NULL;
//    req->state = state;
//
//    work->data = req;
//
//    state->pending_work_count++;
//    int status = uv_queue_work(state->env->loop, (uv_work_t*) work,
//                               request_info,
//                               after_request_info);
//    if (status) {
//        state->error_status = STORJ_QUEUE_ERROR;
//        return;
//    }
//
//}

static uv_work_t *uv_work_new()
{
    uv_work_t *work = malloc(sizeof(uv_work_t));
    return work;
}

static void cleanup_state(storj_download_state_t *state)
{
    state->finished_cb(state->error_status, state->destination, state->handle);

    free(state);
}

static void cleanup_work(uv_work_t *work, int status)
{
    get_file_info_request_t *req = work->data;
    uv_work_t *download_work = req->handle;
    storj_download_state_t *state = download_work->data;

    cleanup_state(state);
    free(download_work);
    storj_free_get_file_info_request(req);
    free(work);
}

static void resolve_file(uv_work_t *work)
{
    get_file_info_request_t *req = work->data;
    uv_work_t *download_work = req->handle;
    storj_download_state_t *state = download_work->data;

    // Load progress bar
    state->progress_cb(0, 0, 0, state->handle);

    BucketRef bucket_ref = open_bucket(state->env->project_ref,
                                     strdup(state->bucket_id),
                                     strdup(state->encryption_access),
                                     STORJ_LAST_ERROR);
    STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR;

    DownloaderRef downloader_ref = download(bucket_ref, strdup(state->file_id),
                                            STORJ_LAST_ERROR);
    STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR;

    state->downloader_ref = downloader_ref;

    size_t buf_len;
    uint8_t *buf;
    while (true) {
        buf = malloc(buf_len);
        size_t read_size = download_read(state->downloader_ref, buf, buf_len, STORJ_LAST_ERROR);
        STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR;
        // TODO: what if read_size != buf_len!?

        if (read_size <= 0) {
            free(buf);
            break;
        }

        size_t written_size = fwrite(buf, sizeof(char), buf_len, state->destination);
        // TODO: what if written_size != buf_len!?

        // TODO: use uv_async_init/uv_async_send instead of calling cb directly?
        state->downloaded_bytes += read_size;
        double progress = state->downloaded_bytes / state->info->size;
        state->progress_cb(progress, state->downloaded_bytes,
                           state->info->size, state->handle);
        free(buf);
    }
    state->progress_cb(1, state->downloaded_bytes,
                       state->info->size, state->handle);

//    state->progress_finished = true;

    download_close(downloader_ref, STORJ_LAST_ERROR);
    STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR;

    state->finished = true;
//    state->finished_cb(state->error_status, state->destination, state->handle);

//    free_download_state(state);
}

static void queue_resolve_file(uv_work_t *work, int status)
{
    get_file_info_request_t *req = work->data;
    uv_work_t *download_work = req->handle;
    storj_download_state_t *state = download_work->data;

    // TODO: need to copy?
    state->info = req->file;

    uv_queue_work(state->env->loop, work, resolve_file, cleanup_work);
}

//STORJ_API int storj_bridge_resolve_file_cancel(storj_download_state_t *state)
//{
//    if (state->canceled) {
//        return 0;
//    }
//
//    state->canceled = true;
//    state->error_status = STORJ_TRANSFER_CANCELED;
//
//    // loop over all pointers, and cancel any that are queued to be downloaded
//    // any downloads that are in-progress will monitor the state->canceled
//    // status and exit when set to true
//    for (int i = 0; i < state->total_pointers; i++) {
//        storj_pointer_t *pointer = &state->pointers[i];
//        if (pointer->status == POINTER_BEING_DOWNLOADED) {
//            uv_cancel((uv_req_t *)pointer->work);
//        }
//    }
//
//    return 0;
//}

STORJ_API storj_download_state_t *storj_bridge_resolve_file(storj_env_t *env,
                                                            const char *bucket_id,
                                                            const char *file_id,
                                                            FILE *destination,
                                                            const char *encryption_access,
                                                            void *handle,
                                                            storj_progress_cb progress_cb,
                                                            storj_finished_download_cb finished_cb)
{
    storj_download_state_t *state = malloc(sizeof(storj_download_state_t));
    if (!state) {
        return NULL;
    }

    // setup download state
    state->encryption_access = strdup(encryption_access);
    state->total_bytes = 0;
    state->downloaded_bytes = 0;
    state->env = env;
    state->file_id = strdup(file_id);
    state->bucket_id = strdup(bucket_id);
    state->destination = destination;
    state->progress_cb = progress_cb;
    state->finished_cb = finished_cb;
    state->finished = false;
    state->error_status = STORJ_TRANSFER_OK;
    state->log = env->log;
    state->handle = handle;

    uv_work_t *work = uv_work_new();
    work->data = state;

    int status = storj_bridge_get_file_info(state->env, state->bucket_id, state->file_id,
                                            strdup(encryption_access), work,
                                            queue_resolve_file);
//    int status = uv_queue_work(env->loop, (uv_work_t*) work,
//                               resolve_file, queue_get_file_info);
    if (status) {
        state->error_status = STORJ_QUEUE_ERROR;
    }
    return state;
}
