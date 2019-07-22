#include "storj.h"

static uv_work_t *uv_work_new()
{
    uv_work_t *work = malloc(sizeof(uv_work_t));
    return work;
}

static void cleanup_state(storj_upload_state_t *state)
{
    if (state->original_file) {
        fclose(state->original_file);
    }

    state->finished_cb(state->error_status, state->info, state->handle);

    free(state);
}

static void cleanup_upload_work(uv_work_t *work)
{
    storj_upload_state_t *state = work->data;

    cleanup_state(state);
    free(work);
}

static void after_get_file_info(uv_work_t *work, int status)
{
    get_file_info_request_t *req = NULL;
    uv_work_t *upload_work;
    storj_upload_state_t *state;

    // NB: hack
    if (status) {
        upload_work = work;
        work = NULL;
        state = upload_work->data;
    } else {
        req = work->data;
        upload_work = req->handle;
        state = upload_work->data;
    }

    if (state->error_status) {
        /* Currently, if status == 0 && state->error_status != 0,
           finished_cb gets called; this is inconsistent. */
        goto cleanup;
    }

    storj_file_meta_t *info = state->info;
    STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR;

    info->filename = strdup(req->file->filename);

    info->created = strdup(req->file->created);
    info->mimetype = strdup(req->file->mimetype);
    info->bucket_id = strdup(req->file->bucket_id);
    info->id = strdup(req->file->id);
    info->size = req->file->size;

cleanup:
    cleanup_upload_work(upload_work);
    if (req) {
        storj_free_get_file_info_request(req);
    }
    if (work) {
        free(work);
    }
}

static void queue_get_file_info(uv_work_t *work, int status)
{
    storj_upload_state_t *state = work->data;
    if (state->error_status) {
        after_get_file_info(work, state->error_status);
        return;
    }

    storj_bridge_get_file_info(state->env, state->bucket_id, state->file_name,
                               strdup(state->encryption_access), work,
                               after_get_file_info);
}

static void store_file(uv_work_t *work)
{
    storj_upload_state_t *state = work->data;

    size_t buf_len;
    uint8_t *buf;
    while (state->uploaded_bytes < state->file_size) {
        size_t remaining_size = state->file_size - state->uploaded_bytes;
        if (remaining_size >= state->buffer_size) {
            buf_len = state->buffer_size;
        } else {
            buf_len = remaining_size;
        }

        buf = malloc(buf_len);
        size_t read_size = fread(buf, sizeof(char), buf_len, state->original_file);
        // TODO: what if read_size != buf_len?

        int written_size = upload_write(state->uploader_ref, buf, read_size, STORJ_LAST_ERROR);
        STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR;
        if (written_size != buf_len) {
            free(buf);
            return;
        }

        // TODO: use uv_async_init/uv_async_send instead of calling cb directly?
        state->uploaded_bytes += written_size;
        double progress = (double)state->uploaded_bytes / state->file_size;
        state->progress_cb(progress, state->uploaded_bytes,
                           state->file_size, state->handle);
        free(buf);
    }

    state->progress_finished = true;

    upload_commit(state->uploader_ref, STORJ_LAST_ERROR);
    STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR;

    state->completed_upload = true;
}

static void prepare_upload_state(uv_work_t *work)
{
    storj_upload_state_t *state = work->data;

    // Get the file size, expect to be up to 10tb
#ifdef _WIN32
    struct _stati64 st;

    if(_fstati64(fileno(state->original_file), &st) != 0) {
        state->error_status = STORJ_FILE_INTEGRITY_ERROR;
        return;
    }
#else
    struct stat st;
    if(fstat(fileno(state->original_file), &st) != 0) {
        state->error_status = STORJ_FILE_INTEGRITY_ERROR;
        return;
    }
#endif

    BucketRef bucket_ref = open_bucket(state->env->project_ref,
                                     strdup(state->bucket_id),
                                     strdup(state->encryption_access),
                                     STORJ_LAST_ERROR);
    STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR;

    UploaderRef uploader_ref = upload(bucket_ref, strdup(state->file_name),
                                      state->upload_opts, STORJ_LAST_ERROR);
    STORJ_RETURN_SET_STATE_ERROR_IF_LAST_ERROR;
    state->uploader_ref = uploader_ref;

    state->file_size = st.st_size;
    state->info = malloc(sizeof(storj_file_meta_t));
    state->info->created = NULL;
    state->info->filename = state->file_name;
    state->info->mimetype = NULL;
    state->info->size = state->file_size;
    state->info->id = NULL;
    state->info->bucket_id = state->bucket_id;
    state->info->decrypted = true;

    state->canceled = false;
    state->completed_upload = false;
    state->progress_finished = false;

    state->progress_cb(0, 0, 0, state->handle);
}

STORJ_API int storj_bridge_store_file_cancel(storj_upload_state_t *state)
{
    if (state->canceled) {
        return 0;
    }

    state->canceled = true;

    state->error_status = STORJ_TRANSFER_CANCELED;

    upload_cancel(state->uploader_ref, STORJ_LAST_ERROR);
    STORJ_RETURN_IF_LAST_ERROR(state->error_status);

    return 0;
}

STORJ_API storj_upload_state_t *storj_bridge_store_file(storj_env_t *env,
                            storj_upload_opts_t *opts,
                            void *handle,
                            storj_progress_cb progress_cb,
                            storj_finished_upload_cb finished_cb)
{
    if (!opts->fd) {
        env->log->error(env->log_options, handle, "Invalid File descriptor");
        return NULL;
    }

    if (opts->expires <= 0) {
        *STORJ_LAST_ERROR = "upload option \"expires\" must be greater than zero.";
        env->log->error(env->log_options, handle, *STORJ_LAST_ERROR);
        return NULL;
    }

    storj_upload_state_t *state = malloc(sizeof(storj_upload_state_t));
    if (!state) {
        return NULL;
    }

    state->buffer_size = (opts->buffer_size == 0) ?
        STORJ_DEFAULT_UPLOAD_BUFFER_SIZE : opts->buffer_size;

    state->upload_opts = malloc(sizeof(UploadOptions));
    state->upload_opts->expires = opts->expires;
    state->upload_opts->content_type = (opts->content_type) ?
        strdup(opts->content_type) : "";

    state->env = env;
    state->file_name = strdup(opts->file_name);
    state->encryption_access = strdup(opts->encryption_access);
    state->file_size = 0;
    state->uploaded_bytes = 0;
    state->bucket_id = strdup(opts->bucket_id);
    state->encrypted_file_name = strdup(opts->file_name);

    state->original_file = opts->fd;

    state->progress_finished = false;

    state->progress_cb = progress_cb;
    state->finished_cb = finished_cb;
    state->error_status = 0;
    state->log = env->log;
    state->handle = handle;

    uv_work_t *work = uv_work_new();
    work->data = state;

    prepare_upload_state(work);
    int status = uv_queue_work(env->loop, work,
                               store_file, queue_get_file_info);
    if (status) {
        state->error_status = STORJ_QUEUE_ERROR;
    }
    return state;
}

STORJ_API void storj_free_uploaded_file_info(storj_file_meta_t *file)
{
    if (file) {
        free((char *)file->id);
        free((char *)file->created);
        free((char *)file->mimetype);
    }
    free(file);
}
