#include <time.h>
#include "storj.h"

char *_storj_last_error = "";
char **STORJ_LAST_ERROR = &_storj_last_error;

static inline void noop() {};

static void create_bucket_request_worker(uv_work_t *work)
{
    create_bucket_request_t *req = work->data;

    BucketInfo *created_bucket = malloc(sizeof(BucketInfo));
    *created_bucket = create_bucket(req->project_ref,
                                    strdup(req->bucket_name),
                                    req->bucket_cfg, STORJ_LAST_ERROR);
    STORJ_RETURN_SET_REQ_ERROR_IF_LAST_ERROR;

    char created_str[32];
    time_t created_time = (time_t)created_bucket->created;
    strftime(created_str, 32, "%DT%T%Z", localtime(&created_time));

    req->bucket_name = strdup(created_bucket->name);
    req->bucket = malloc(sizeof(storj_bucket_meta_t));

    req->bucket->name = strdup(created_bucket->name);
    req->bucket->id = strdup(created_bucket->name);
    req->bucket->created = strdup(created_str);
    req->bucket->decrypted = true;
    // NB: this field is unused; it only exists for backwards compatibility as it is
    //  passed to `json_object_put` by api consumers.
    //  (see: https://svn.filezilla-project.org/svn/FileZilla3/trunk/src/storj/fzstorj.cpp)
    req->response = json_object_new_object();

    free_bucket_info((BucketInfo *)&created_bucket);
}

static void get_buckets_request_worker(uv_work_t *work)
{
    get_buckets_request_t *req = work->data;

    BucketList bucket_list = list_buckets(req->project_ref, NULL, STORJ_LAST_ERROR);
    STORJ_RETURN_SET_REQ_ERROR_IF_LAST_ERROR;

    req->total_buckets = bucket_list.length;

    if (bucket_list.length > 0) {
        req->buckets = malloc(sizeof(storj_bucket_meta_t) * bucket_list.length);

        BucketInfo bucket_item;
        for (int i = 0; i < bucket_list.length; i++) {
            bucket_item = bucket_list.items[i];
            storj_bucket_meta_t *bucket = &req->buckets[i];

            char created_str[32];
            time_t created_time = (time_t)bucket_item.created;
            strftime(created_str, 32, "%DT%T%Z", localtime(&created_time));

            bucket->name = strdup(bucket_item.name);
            bucket->id = strdup(bucket_item.name);
            bucket->created = strdup(created_str);
            bucket->decrypted = true;
        }
    }

    // NB: this field is unused; it only exists for backwards compatibility as it is
    //  passed to `json_object_put` by api consumers.
    //  (see: https://svn.filezilla-project.org/svn/FileZilla3/trunk/src/storj/fzstorj.cpp)
    req->response = json_object_new_object();

    free_bucket_list(&bucket_list);
}

static void get_bucket_request_worker(uv_work_t *work)
{
    get_bucket_request_t *req = work->data;

    BucketInfo bucket_info = get_bucket_info(req->project_ref,
                                             strdup(req->bucket_name),
                                             STORJ_LAST_ERROR);
    STORJ_RETURN_SET_REQ_ERROR_IF_LAST_ERROR

    req->bucket = malloc(sizeof(storj_bucket_meta_t));

    char created_str[32];
    time_t created_time = (time_t)bucket_info.created;
    strftime(created_str, 32, "%DT%T%Z", localtime(&created_time));

    req->bucket->name = strdup(bucket_info.name);
    req->bucket->id = strdup(bucket_info.name);
    req->bucket->created = strdup(created_str);
    req->bucket->decrypted = true;
    // NB: this field is unused; it only exists for backwards compatibility as it is
    //  passed to `json_object_put` by api consumers.
    //  (see: https://svn.filezilla-project.org/svn/FileZilla3/trunk/src/storj/fzstorj.cpp)
    req->response = json_object_new_object();

    free_bucket_info((BucketInfo *)&bucket_info);
}

static void delete_bucket_request_worker(uv_work_t *work)
{
    delete_bucket_request_t *req = work->data;

    delete_bucket(req->project_ref, strdup(req->bucket_name), STORJ_LAST_ERROR);
    STORJ_RETURN_SET_REQ_ERROR_IF_LAST_ERROR

    // NB: http "no content" success status code.
    req->status_code = 204;
}

static void list_files_request_worker(uv_work_t *work)
{
    list_files_request_t *req = work->data;

    BucketRef bucket_ref = open_bucket(req->project_ref, strdup(req->bucket_id),
                                       strdup(req->encryption_access),
                                       STORJ_LAST_ERROR);
    STORJ_RETURN_SET_REQ_ERROR_IF_LAST_ERROR;

    ObjectList object_list = list_objects(bucket_ref, NULL, STORJ_LAST_ERROR);
    STORJ_RETURN_SET_REQ_ERROR_IF_LAST_ERROR;

    req->total_files = object_list.length;

    if (object_list.length > 0) {
        req->files = malloc(sizeof(storj_file_meta_t) * object_list.length);

        ObjectInfo object_item;
        for (int i = 0; i < object_list.length; i++) {
            object_item = object_list.items[i];
            storj_file_meta_t *file = &req->files[i];

            char created_str[32];
            time_t created_time = (time_t)object_item.created;
            strftime(created_str, 32, "%DT%T%Z", localtime(&created_time));

            file->created = strdup(created_str);
            file->mimetype = strdup(object_item.content_type);
            file->id = strdup(object_item.path);
            file->bucket_id = strdup(object_item.bucket.name);
            file->filename = strdup(object_item.path);
            file->decrypted = true;

            // TODO: if we want to populate size we need to
            //  get object meta for each file.
//            file->size = ;
        }
    }
}

static void get_file_info_request_worker(uv_work_t *work)
{
    get_file_info_request_t *req = work->data;

    ObjectRef object_ref = open_object(req->bucket_ref, strdup(req->path), STORJ_LAST_ERROR);
    STORJ_RETURN_SET_REQ_ERROR_IF_LAST_ERROR;

    ObjectMeta object_meta = get_object_meta(object_ref, STORJ_LAST_ERROR);
    STORJ_RETURN_SET_REQ_ERROR_IF_LAST_ERROR;

    req->file = malloc(sizeof(storj_file_meta_t));

    char created_str[32];
    time_t created_time = (time_t)object_meta.created;
    strftime(created_str, 32, "%DT%T%Z", localtime(&created_time));

    req->file->created = strdup(created_str);
    req->file->mimetype = strdup(object_meta.content_type);
    req->file->size = (int64_t)object_meta.size;
    req->file->id = strdup(object_meta.path);
    req->file->bucket_id = strdup(object_meta.bucket);
    req->file->filename = strdup(object_meta.path);
    req->file->decrypted = true;
}

static uv_work_t *uv_work_new()
{
    uv_work_t *work = malloc(sizeof(uv_work_t));
    return work;
}

static list_files_request_t *list_files_request_new(
    ProjectRef project_ref,
    const char *bucket_id,
    const char *encryption_access,
    void *handle)
{
    list_files_request_t *req = malloc(sizeof(list_files_request_t));
    if (!req) {
        return NULL;
    }

    req->project_ref = project_ref;
    req->bucket_id = strdup(bucket_id);
    req->encryption_access = strdup(encryption_access);
    req->response = NULL;
    req->files = NULL;
    req->total_files = 0;
    req->error_code = 0;
    req->status_code = 0;
    req->handle = handle;

    return req;
}

static get_file_info_request_t *get_file_info_request_new(
    ProjectRef project_ref,
    const char *bucket_id,
    const char *path,
    const char *encryption_access,
    void *handle)
{
    BucketRef bucket_ref = open_bucket(project_ref, strdup(bucket_id), strdup(encryption_access), STORJ_LAST_ERROR);

    get_file_info_request_t *req = malloc(sizeof(get_file_info_request_t));
    if (!req) {
        return NULL;
    }

    req->bucket_ref = bucket_ref;
    req->bucket_id = bucket_id;
    req->path = strdup(path);
    req->response = NULL;
    req->file = NULL;
    req->error_code = 0;
    req->status_code = 0;
    req->handle = handle;

    return req;
}

static create_bucket_request_t *create_bucket_request_new(
    ProjectRef project_ref,
    const char *bucket_name,
    BucketConfig *cfg,
    void *handle)
{
    create_bucket_request_t *req = malloc(sizeof(create_bucket_request_t));
    if (!req) {
        return NULL;
    }

    req->bucket_cfg = cfg;
    req->bucket_name = strdup(bucket_name);
    req->project_ref = project_ref;
    req->response = NULL;
    req->bucket = NULL;
    req->error_code = 0;
    req->status_code = 0;
    req->handle = handle;

    return req;
}

static get_buckets_request_t *get_buckets_request_new(
    ProjectRef project_ref,
    void *handle)
{
    get_buckets_request_t *req = malloc(sizeof(get_buckets_request_t));
    if (!req) {
        return NULL;
    }

    req->project_ref = project_ref;
    req->response = NULL;
    req->buckets = NULL;
    req->total_buckets = 0;
    req->error_code = 0;
    req->status_code = 0;
    req->handle = handle;

    return req;
}

static get_bucket_request_t *get_bucket_request_new(
        ProjectRef project_ref,
        char *bucket_name,
        void *handle)
{
    get_bucket_request_t *req = malloc(sizeof(get_bucket_request_t));
    if (!req) {
        return NULL;
    }

    req->project_ref = project_ref;
    req->bucket_name = strdup(bucket_name);
    req->response = NULL;
    req->bucket = NULL;
    req->error_code = 0;
    req->status_code = 0;
    req->handle = handle;

    return req;
}

static get_bucket_id_request_t *get_bucket_id_request_new(
        const char *bucket_name,
        void *handle)
{
    get_bucket_id_request_t *req = malloc(sizeof(get_bucket_id_request_t));
    if (!req) {
        return NULL;
    }

    req->bucket_name = strdup(bucket_name);
    req->bucket_id = strdup(bucket_name);
    req->response = NULL;
    req->error_code = 0;
    req->status_code = 0;
    req->handle = handle;

    return req;
}

static delete_bucket_request_t *delete_bucket_request_new(
        ProjectRef project_ref,
        const char *bucket_name,
        void *handle)
{
    delete_bucket_request_t *req = malloc(sizeof(delete_bucket_request_t));
    if (!req) {
        return NULL;
    }

    req->project_ref = project_ref;
    req->bucket_name = strdup(bucket_name);
    req->response = NULL;
    req->error_code = 0;
    req->status_code = 0;
    req->handle = handle;

    return req;
}

static get_file_id_request_t *get_file_id_request_new(
        const char *bucket_id,
        const char *file_name,
        void *handle)
{
    get_file_id_request_t *req = malloc(sizeof(get_file_id_request_t));
    if (!req) {
        return NULL;
    }

    req->bucket_id = strdup(bucket_id);
    req->file_name = strdup(file_name);
    req->response = NULL;
    req->file_id = strdup(file_name);
    req->error_code = 0;
    req->status_code = 0;
    req->handle = handle;

    return req;
}

static void default_logger(const char *message,
                           int level,
                           void *handle)
{
    puts(message);
}

static void log_formatter(storj_log_options_t *options,
                          void *handle,
                          int level,
                          const char *format,
                          va_list args)
{
    va_list args_cpy;
    va_copy(args_cpy, args);
    int length = vsnprintf(0, 0, format, args_cpy);
    va_end(args_cpy);

    if (length > 0) {
        char message[length + 1];
        if (vsnprintf(message, length + 1, format, args)) {
            options->logger(message, level, handle);
        }
    }
}

static void log_formatter_debug(storj_log_options_t *options, void *handle,
                                const char *format, ...)
{
    va_list args;
    va_start(args, format);
    log_formatter(options, handle, 4, format, args);
    va_end(args);
}

static void log_formatter_info(storj_log_options_t *options, void *handle,
                               const char *format, ...)
{
    va_list args;
    va_start(args, format);
    log_formatter(options, handle, 3, format, args);
    va_end(args);
}

static void log_formatter_warn(storj_log_options_t *options, void *handle,
                               const char *format, ...)
{
    va_list args;
    va_start(args, format);
    log_formatter(options, handle, 2, format, args);
    va_end(args);
}

static void log_formatter_error(storj_log_options_t *options, void *handle,
                                const char *format, ...)
{
    va_list args;
    va_start(args, format);
    log_formatter(options, handle, 1, format, args);
    va_end(args);
}


// TODO: use memlock for encryption and api keys
// (see: https://github.com/storj/libstorj/blob/master/src/storj.c#L853)
STORJ_API storj_env_t *storj_init_env(storj_bridge_options_t *bridge_options,
                                 storj_encrypt_options_t *encrypt_options,
                                 storj_http_options_t *http_options,
                                 storj_log_options_t *log_options)
{
    APIKeyRef apikey_ref = parse_api_key(strdup(bridge_options->apikey), STORJ_LAST_ERROR);
    STORJ_RETURN_IF_LAST_ERROR(NULL);

    UplinkConfig uplink_cfg = {{0}};
    uplink_cfg.Volatile.tls.skip_peer_ca_whitelist = true;

    UplinkRef uplink_ref = new_uplink(uplink_cfg, STORJ_LAST_ERROR);
    STORJ_RETURN_IF_LAST_ERROR(NULL);

    ProjectRef project_ref = open_project(uplink_ref, strdup(bridge_options->addr), apikey_ref, STORJ_LAST_ERROR);
    STORJ_RETURN_IF_LAST_ERROR(NULL);

    storj_env_t *env = malloc(sizeof(storj_env_t));
    env->bridge_options = bridge_options;
    env->encrypt_options = encrypt_options;
    env->http_options = http_options;
    env->log_options = log_options;
    env->uplink_ref = uplink_ref;
    env->project_ref = project_ref;

    uv_loop_t *loop = uv_default_loop();
    if (!loop) {
        return NULL;
    }

    // setup the uv event loop
    env->loop = loop;

    // setup the log options
    env->log_options = log_options;
    if (!env->log_options->logger) {
        env->log_options->logger = default_logger;
    }

    storj_log_levels_t *log = malloc(sizeof(storj_log_levels_t));
    if (!log) {
        return NULL;
    }

    log->debug = (storj_logger_format_fn)noop;
    log->info = (storj_logger_format_fn)noop;
    log->warn = (storj_logger_format_fn)noop;
    log->error = (storj_logger_format_fn)noop;

    switch(log_options->level) {
        case 4:
            log->debug = log_formatter_debug;
        case 3:
            log->info = log_formatter_info;
        case 2:
            log->warn = log_formatter_warn;
        case 1:
            log->error = log_formatter_error;
        case 0:
            break;
    }

    env->log = log;

    return env;
}

// TODO: use memlock for encryption and api keys
// (see: https://github.com/storj/libstorj/blob/master/src/storj.c#L999)
STORJ_API int storj_destroy_env(storj_env_t *env)
{
    close_project(env->project_ref, STORJ_LAST_ERROR);
    STORJ_RETURN_IF_LAST_ERROR(1);

    close_uplink(env->uplink_ref, STORJ_LAST_ERROR);
    STORJ_RETURN_IF_LAST_ERROR(1);

    return 0;
}

STORJ_API char *storj_strerror(int error_code)
{
    switch(error_code) {

        case STORJ_TRANSFER_OK:
            return "No errors";
        case STORJ_TRANSFER_CANCELED:
            return "File transfer canceled";
        case STORJ_LIBUPLINK_ERROR:
            return *STORJ_LAST_ERROR;
        case STORJ_MEMORY_ERROR:
            return "Memory error";
        default:
            return "Unknown error";
    }
}

STORJ_API int storj_bridge_get_buckets(storj_env_t *env, void *handle, uv_after_work_cb cb)
{
    uv_work_t *work = uv_work_new();
    if (!work) {
        return STORJ_MEMORY_ERROR;
    }

    work->data = get_buckets_request_new(env->project_ref, handle);
    if (!work->data) {
        return STORJ_MEMORY_ERROR;
    }

    return uv_queue_work(env->loop, (uv_work_t*) work,
                         get_buckets_request_worker, cb);
}

STORJ_API void storj_free_get_buckets_request(get_buckets_request_t *req)
{
    if (req->response) {
        json_object_put(req->response);
    }
    if (req->buckets && req->total_buckets > 0) {
        for (int i = 0; i < req->total_buckets; i++) {
            free((char *)req->buckets[i].name);
            free((char *)req->buckets[i].id);
            free((char *)req->buckets[i].created);
        }
    }

    free(req->buckets);
    free(req);
}

STORJ_API int storj_bridge_create_bucket(storj_env_t *env,
                               const char *name,
                               BucketConfig *cfg,
                               void *handle,
                               uv_after_work_cb cb)
{
    uv_work_t *work = uv_work_new(); if (!work) {
        return STORJ_MEMORY_ERROR;
    }


    work->data = create_bucket_request_new(env->project_ref,
                                           name,
                                           cfg,
                                           handle);
    if (!work->data) {
        return STORJ_MEMORY_ERROR;
    }

    return uv_queue_work(env->loop, (uv_work_t*) work,
                         create_bucket_request_worker, cb);
}

STORJ_API int storj_bridge_delete_bucket(storj_env_t *env,
                               const char *bucket_name,
                               void *handle,
                               uv_after_work_cb cb)
{
    uv_work_t *work = uv_work_new(); if (!work) {
        return STORJ_MEMORY_ERROR;
    }

    work->data = delete_bucket_request_new(env->project_ref, bucket_name, handle);
    if (!work->data) {
        return STORJ_MEMORY_ERROR;
    }

    return uv_queue_work(env->loop, (uv_work_t*) work, delete_bucket_request_worker, cb);
}

STORJ_API int storj_bridge_get_bucket(storj_env_t *env,
                                      const char *name,
                                      void *handle,
                                      uv_after_work_cb cb)
{
    uv_work_t *work = uv_work_new();
    if (!work) {
        return STORJ_MEMORY_ERROR;
    }

    char *bucket_name = strdup(name);
    work->data = get_bucket_request_new(env->project_ref, bucket_name, handle);
    if (!work->data) {
        return STORJ_MEMORY_ERROR;
    }

    return uv_queue_work(env->loop, (uv_work_t*) work, get_bucket_request_worker, cb);
}

STORJ_API void storj_free_get_bucket_request(get_bucket_request_t *req)
{
    if (req->response) {
        json_object_put(req->response);
    }
    if (req->bucket) {
        free((char *)req->bucket->name);
        free((char *)req->bucket->id);
        free((char *)req->bucket->created);
    }

    free(req->bucket);
    free((char *)req->bucket_name);
    free(req);
}

STORJ_API void storj_free_create_bucket_request(create_bucket_request_t *req)
{
    if (req->response) {
        json_object_put(req->response);
    }
    if (req->bucket) {
        free((char *)req->bucket->name);
        free((char *)req->bucket->id);
        free((char *)req->bucket->created);
    }

    free(req->bucket);
    free((char *)req->bucket_name);
    free(req);
}

STORJ_API int storj_bridge_get_bucket_id(storj_env_t *env,
                                         const char *name,
                                         void *handle,
                                         uv_after_work_cb cb)
{
    uv_work_t *work = uv_work_new();
    if (!work) {
        return STORJ_MEMORY_ERROR;
    }

    work->data = get_bucket_id_request_new(name, handle);
    if (!work->data) {
        return STORJ_MEMORY_ERROR;
    }

    cb(work, 0);
    return 0;
}

STORJ_API int storj_bridge_list_files(storj_env_t *env,
                            const char *id,
                            const char *encryption_access,
                            void *handle,
                            uv_after_work_cb cb)
{
    uv_work_t *work = uv_work_new();
    if (!work) {
        return STORJ_MEMORY_ERROR;
    }
    work->data = list_files_request_new(env->project_ref, id,
                                        encryption_access, handle);

    if (!work->data) {
        return STORJ_MEMORY_ERROR;
    }

    return uv_queue_work(env->loop, (uv_work_t*) work,
                         list_files_request_worker, cb);
}

STORJ_API void storj_free_list_files_request(list_files_request_t *req)
{
    if (req->response) {
        json_object_put(req->response);
    }
    // TODO: either add locking or at lease zero memory out.
    free((char *)req->encryption_access);
    free((char *)req->bucket_id);
    if (req->files && req->total_files > 0) {
        for (int i = 0; i < req->total_files; i++) {
            storj_free_file_meta(&req->files[i]);
        }
    }
    free(req);
}

STORJ_API void storj_free_file_meta(storj_file_meta_t *file_meta)
{
    free((char *)file_meta->filename);
    free((char *)file_meta->bucket_id);
    free((char *)file_meta->mimetype);
    free((char *)file_meta->created);
    free((char *)file_meta->id);
    free(file_meta);
}

//STORJ_API int storj_bridge_delete_file(storj_env_t *env,
//                             const char *bucket_id,
//                             const char *file_id,
//                             void *handle,
//                             uv_after_work_cb cb)
//{
//    char *path = str_concat_many(4, "/buckets/", bucket_id, "/files/", file_id);
//    if (!path) {
//        return STORJ_MEMORY_ERROR;
//    }
//
//    uv_work_t *work = json_request_work_new(env, "DELETE", path, NULL,
//                                            true, handle);
//    if (!work) {
//        return STORJ_MEMORY_ERROR;
//    }
//
//    return uv_queue_work(env->loop, (uv_work_t*) work, json_request_worker, cb);
//}

STORJ_API int storj_bridge_get_file_info(storj_env_t *env,
                                         const char *bucket_id,
                                         const char *file_id,
                                         const char *encryption_access,
                                         void *handle,
                                         uv_after_work_cb cb)
{
    uv_work_t *work = uv_work_new();
    if (!work) {
        return STORJ_MEMORY_ERROR;
    }

    work->data = get_file_info_request_new(env->project_ref, bucket_id,
                                           file_id, encryption_access, handle);

    if (!work->data) {
        return STORJ_MEMORY_ERROR;
    }

    return uv_queue_work(env->loop, (uv_work_t*) work,
                         get_file_info_request_worker, cb);
}

STORJ_API int storj_bridge_get_file_id(storj_env_t *env,
                                       const char *bucket_id,
                                       const char *file_name,
                                       void *handle,
                                       uv_after_work_cb cb)
{
    uv_work_t *work = uv_work_new();
    if (!work) {
        return STORJ_MEMORY_ERROR;
    }

    work->data = get_file_id_request_new(bucket_id, file_name, handle);
    if (!work->data) {
        return STORJ_MEMORY_ERROR;
    }

    cb(work, 0);
    return 0;
}

STORJ_API void storj_free_get_file_info_request(get_file_info_request_t *req)
{
    if (req->response) {
        json_object_put(req->response);
    }
    free(req->path);
    if (req->file) {
        storj_free_file_meta(req->file);
    }
    free(req);
}
