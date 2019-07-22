#include "storj.h"

static void free_download_state(storj_download_state_t *state)
{
    // TODO: free `state->info` & ptrs if `queue_resolve_file` duplicates the file meta.
    free(state);
}

static uv_work_t *uv_work_new()
{
    uv_work_t *work = malloc(sizeof(uv_work_t));
    return work;
}

static void cleanup_state(storj_download_state_t *state)
{
    state->finished_cb(state->error_status, state->destination, state->handle);

    free_download_state(state);
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
    while (state->downloaded_bytes < state->total_bytes) {
        size_t remaining_size = state->total_bytes - state->downloaded_bytes;
        if (remaining_size >= state->buffer_size) {
            buf_len = state->buffer_size;
        } else {
            buf_len = remaining_size;
        }

        buf = malloc(buf_len);
        size_t read_size = download_read(state->downloader_ref, buf, buf_len, STORJ_LAST_ERROR);
        STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR;
        // TODO: what if read_size != buf_len!?

        if (read_size <= 0) {
            free(buf);
            // TODO: call finished_cb?
            break;
        }

        size_t written_size = fwrite(buf, sizeof(char), read_size, state->destination);
        // TODO: what if written_size != buf_len!?

        // TODO: use uv_async_init/uv_async_send instead of calling cb directly?
        state->downloaded_bytes += read_size;
        double progress = (double)state->downloaded_bytes / state->total_bytes;
        state->progress_cb(progress, state->downloaded_bytes,
                           state->total_bytes, state->handle);
        free(buf);
    }

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

    if (state->canceled) {
        cleanup_work(work, state->error_status);
        return;
    }

    // TODO: we should really copy the struct & free it in `free_download_state`
    state->info = req->file;
    state->total_bytes = req->file->size;

    uv_queue_work(state->env->loop, work, resolve_file, cleanup_work);
}

STORJ_API int storj_bridge_resolve_file_cancel(storj_download_state_t *state)
{
    if (state->canceled) {
        return 0;
    }

    state->canceled = true;
    state->error_status = STORJ_TRANSFER_CANCELED;

    if (state->downloader_ref._handle) {
        download_cancel(state->downloader_ref, STORJ_LAST_ERROR);
        STORJ_RETURN_IF_LAST_ERROR(state->error_status);
    }

    return 0;

}

STORJ_API storj_download_state_t *storj_bridge_resolve_file(storj_env_t *env,
                                                            const char *bucket_id,
                                                            const char *file_id,
                                                            FILE *destination,
                                                            const char *encryption_access,
                                                            size_t buffer_size,
                                                            void *handle,
                                                            storj_progress_cb progress_cb,
                                                            storj_finished_download_cb finished_cb)
{
    storj_download_state_t *state = malloc(sizeof(storj_download_state_t));
    if (!state) {
        return NULL;
    }

    state->buffer_size = (buffer_size == 0) ?
        STORJ_DEFAULT_DOWNLOAD_BUFFER_SIZE : buffer_size;

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
    state->canceled = false;
    state->finished = false;
    state->error_status = STORJ_TRANSFER_OK;
    state->log = env->log;
    state->handle = handle;

    state->progress_cb(0, 0, 0, state->handle);

    uv_work_t *work = uv_work_new();
    work->data = state;

    int status = storj_bridge_get_file_info(state->env, state->bucket_id, state->file_id,
                                            strdup(encryption_access), work,
                                            queue_resolve_file);
    if (status) {
        state->error_status = STORJ_QUEUE_ERROR;
    }
    return state;
}
