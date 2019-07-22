#include "cli_callback.h"
#include <dirent.h>

//#define debug_enable

#define MAX_UPLOAD_FILES        256

static inline void noop() {};

static void get_input(char *line)
{
    if (fgets(line, BUFSIZ, stdin) == NULL) {
        line[0] = '\0';
    } else {
        int len = strlen(line);
        if (len > 0) {
            char *last = strrchr(line, '\n');
            if (last) {
                last[0] = '\0';
            }
            last = strrchr(line, '\r');
            if (last) {
                last[0] = '\0';
            }
        }
    }
}

/* inserts into subject[] at position pos */
void append(char subject[], const char insert[], int pos)
{
    char buf[256] = { };

    /* copy at most first pos characters */
    strncpy(buf, subject, pos);
    int len = strlen(buf);

    /* copy all of insert[] at the end */
    strcpy(buf + len, insert);

    /* increase the length by length of insert[] */
    len += strlen(insert);

    /* copy the rest */
    strcpy(buf + len, subject + pos);

    /* copy it back to subject */
    strcpy(subject, buf);
}

char* replace_char(char* str, char find, char replace)
{
    char *current_pos = strchr(str, find);
    while (current_pos) {
        *current_pos = replace;
        append(current_pos, "_", 0);
        current_pos = strchr(current_pos, find);
    }
    return (str);
}

static void printdir(char *dir, int depth, FILE *src_fd, void *handle)
{
    DIR *dp;
    struct dirent *entry;
    struct stat statbuf;

    if ((dp = opendir(dir)) == NULL) {
        fprintf(stderr,"cannot open directory: %s\n", dir);
        return;
    }

    int ret = chdir(dir);
    while ((entry = readdir(dp)) != NULL) {
        stat(entry->d_name, &statbuf);
        if (S_ISDIR(statbuf.st_mode)) {
            /* Found a directory, but ignore . and .. */
            if (strcmp(".", entry->d_name) == 0 ||
                strcmp("..", entry->d_name) == 0) continue;

            /* Recurse at a new indent level */
            printdir(entry->d_name, depth + 1, src_fd, handle);
        } else {
            /* write to src file */
            fprintf(src_fd, "%s%s\n", "", entry->d_name);
        }
    }
    ret = chdir("..");
    closedir(dp);
}

static int file_exists(void *handle)
{
    struct stat sb;
    cli_api_t *cli_api = handle;

    FILE *src_fd, *dst_fd;

    if (stat(cli_api->file_path, &sb) == -1) {
        perror("stat");
        return CLI_NO_SUCH_FILE_OR_DIR;
    }

    switch (sb.st_mode & S_IFMT) {
        case S_IFBLK:
            printf("block device\n");
            break;
        case S_IFCHR:
            printf("character device\n");
            break;
        case S_IFDIR:
            if ((src_fd = fopen(cli_api->src_list, "w")) == NULL) {
                return CLI_UPLOAD_FILE_LOG_ERR;
            }
            printdir(cli_api->file_path, 0, src_fd, handle);
            fclose(src_fd);
            return CLI_VALID_DIR;
            break;
        case S_IFIFO:
            printf("FIFO/pipe\n");
            break;
        case S_IFLNK:
            printf("symlink\n");
            break;
        case S_IFREG:
            return CLI_VALID_REGULAR_FILE;
            break;
#ifdef S_IFSOCK
        case S_IFSOCK:
            printf("socket\n");
            break;
#endif
        default:
            printf("unknown?\n");
            break;
    }

    return CLI_UNKNOWN_FILE_ATTR;
}

static const char *get_filename_separator(const char *file_path)
{
    const char *file_name = NULL;
#ifdef _WIN32
    file_name = strrchr(file_path, '\\');
    if (!file_name) {
        file_name = strrchr(file_path, '/');
    }
    if (!file_name && file_path) {
        file_name = file_path;
    }
    if (!file_name) {
        return NULL;
    }
    if (file_name[0] == '\\' || file_name[0] == '/') {
        file_name++;
    }
#else
    file_name = strrchr(file_path, '/');
    if (!file_name && file_path) {
        file_name = file_path;
    }
    if (!file_name) {
        return NULL;
    }
    if (file_name[0] == '/') {
        file_name++;
    }
#endif
    return file_name;
}

static void close_signal(uv_handle_t *handle)
{
    ((void)0);
}

static void file_progress(double progress,
                          uint64_t downloaded_bytes,
                          uint64_t total_bytes,
                          void *handle)
{
    int bar_width = 70;

    if (progress == 0 && downloaded_bytes == 0) {
        printf("Preparing File...");
        fflush(stdout);
        return;
    }

    printf("\r[");
    int pos = bar_width * progress;
    for (int i = 0; i < bar_width; ++i) {
        if (i < pos) {
            printf("=");
        }
        else if (i == pos) {
            printf(">");
        } else {
            printf(" ");
        }
    }
    printf("] %.*f%%", 2, progress * 100);

    fflush(stdout);
}

static void upload_file_complete(int status, storj_file_meta_t *file, void *handle)
{
    cli_api_t *cli_api = handle;
    cli_api->rcvd_cmd_resp = "upload-file-resp";

    printf("\n");
    if (status != 0) {
        printf("Upload failure: %s\n", storj_strerror(status));
        exit(status);
    }

    printf("Upload Success! File ID: %s\n", file->id);

    storj_free_uploaded_file_info(file);

    queue_next_cmd_req(cli_api);
}

static void upload_signal_handler(uv_signal_t *req, int signum)
{
    storj_upload_state_t *state = req->data;
    storj_bridge_store_file_cancel(state);
    if (uv_signal_stop(req)) {
        printf("Unable to stop signal\n");
    }
    uv_close((uv_handle_t *)req, close_signal);
}

static int upload_file(storj_env_t *env, char *bucket_id, const char *file_path, void *handle)
{
    cli_api_t *cli_api = handle;

    FILE *fd = fopen(file_path, "r");

    if (!fd) {
        printf("Invalid file path: %s\n", file_path);
        exit(0);
    }

    const char *file_name = get_filename_separator(file_path);

    if (cli_api->dst_file == NULL) {
        if (!file_name) {
            file_name = file_path;
        }
    } else {
        file_name = cli_api->dst_file;
    }

    // Upload opts env variables:
    char *prepare_frame_limit = getenv("STORJ_PREPARE_FRAME_LIMIT");
    char *push_frame_limit = getenv("STORJ_PUSH_FRAME_LIMIT");
    char *push_shard_limit = getenv("STORJ_PUSH_SHARD_LIMIT");
    char *rs = getenv("STORJ_REED_SOLOMON");

    storj_upload_opts_t upload_opts = {
        .prepare_frame_limit = (prepare_frame_limit) ? atoi(prepare_frame_limit) : 1,
        .push_frame_limit = (push_frame_limit) ? atoi(push_frame_limit) : 64,
        .push_shard_limit = (push_shard_limit) ? atoi(push_shard_limit) : 64,
        .rs = (!rs) ? true : (strcmp(rs, "false") == 0) ? false : true,
        .bucket_id = bucket_id,
        .file_name = file_name,
        .fd = fd
    };

    uv_signal_t *sig = malloc(sizeof(uv_signal_t));
    if (!sig) {
        return 1;
    }
    uv_signal_init(env->loop, sig);
    uv_signal_start(sig, upload_signal_handler, SIGINT);



    storj_progress_cb progress_cb = (storj_progress_cb)noop;
    if (env->log_options->level == 0) {
        progress_cb = file_progress;
    }

    storj_upload_state_t *state = storj_bridge_store_file(env,
                                                          &upload_opts,
                                                          handle,
                                                          progress_cb,
                                                          upload_file_complete);

    if (!state) {
        return 1;
    }

    sig->data = state;

    return state->error_status;
}

static void upload_files_complete(int status, storj_file_meta_t *file, void *handle)
{
    cli_api_t *cli_api = handle;
    cli_api->rcvd_cmd_resp = "upload-files-resp";

    printf("\n");
    if (status != 0) {
        printf("[%s][%d]Upload failure: %s\n",
               __FUNCTION__, __LINE__, storj_strerror(status));
    } else {
        printf("Upload Success! File ID: %s\n", file->id);
        storj_free_uploaded_file_info(file);
    }

    queue_next_cmd_req(cli_api);
}

static int upload_files(storj_env_t *env, char *bucket_id, const char *file_path, void *handle)
{
    cli_api_t *cli_api = handle;

    FILE *fd = fopen(file_path, "r");

    if (!fd) {
        printf("[%s][%d]Invalid file : %s\n", __FUNCTION__, __LINE__, file_path);
        exit(0);
    }

    printf("Uploading[%d]of[%d] src file = %s as ",
           cli_api->xfer_count, cli_api->total_files, file_path);

    /* replace the dir with __ */
    char *s = strstr(cli_api->src_file, cli_api->file_path);
    char *start = s + strlen(cli_api->file_path);
    char tmp_dir[256];
    start = replace_char(start, '/', '_');
    memset(tmp_dir, 0x00, sizeof(tmp_dir));
    strcat(tmp_dir, cli_api->file_path);
    strcat(tmp_dir, start);
    cli_api->dst_file = tmp_dir;

    const char *file_name = get_filename_separator(cli_api->dst_file);

    if (!file_name) {
        file_name = file_path;
    }
    printf(" %s\n", file_name);

    // Upload opts env variables:
    char *prepare_frame_limit = getenv("STORJ_PREPARE_FRAME_LIMIT");
    char *push_frame_limit = getenv("STORJ_PUSH_FRAME_LIMIT");
    char *push_shard_limit = getenv("STORJ_PUSH_SHARD_LIMIT");
    char *rs = getenv("STORJ_REED_SOLOMON");

    storj_upload_opts_t upload_opts = {
        .prepare_frame_limit = (prepare_frame_limit) ? atoi(prepare_frame_limit) : 1,
        .push_frame_limit = (push_frame_limit) ? atoi(push_frame_limit) : 64,
        .push_shard_limit = (push_shard_limit) ? atoi(push_shard_limit) : 64,
        .rs = (!rs) ? true : (strcmp(rs, "false") == 0) ? false : true,
        .bucket_id = bucket_id,
        .file_name = file_name,
        .fd = fd
    };

    uv_signal_t *sig = malloc(sizeof(uv_signal_t));
    if (!sig) {
        return 1;
    }
    uv_signal_init(env->loop, sig);
    uv_signal_start(sig, upload_signal_handler, SIGINT);

    storj_progress_cb progress_cb = (storj_progress_cb)noop;
    if (env->log_options->level == 0) {
        progress_cb = file_progress;
    }

    storj_upload_state_t *state = storj_bridge_store_file(env,
                                                          &upload_opts,
                                                          handle,
                                                          progress_cb,
                                                          upload_files_complete);

    if (!state) {
        return 1;
    }

    sig->data = state;

    return state->error_status;
}

static void verify_upload_files(void *handle)
{
    cli_api_t *cli_api = handle;
    int total_src_files = 0x00;
    int total_dst_files = 0x00;
    int ret = 0x00;

    /* upload list file previously not created? */
    if (cli_api->dst_file == NULL)
    {
        char pwd_path[256]= {};
        memset(pwd_path, 0x00, sizeof(pwd_path));
        char *upload_list_file = pwd_path;

        /* create upload files list based on the file path */
        if ((upload_list_file = getenv("TMPDIR")) != NULL) {
            if (upload_list_file[(strlen(upload_list_file) - 1)] == '/') {
                strcat(upload_list_file, "STORJ_output_list.txt");
            } else {
                strcat(upload_list_file, "/STORJ_output_list.txt");
            }

            /* check the directory and create the path to upload list file */
            memset(cli_api->src_list, 0x00, sizeof(cli_api->src_list));
            memcpy(cli_api->src_list, upload_list_file, sizeof(pwd_path));
            cli_api->dst_file = cli_api->src_list;
        }

        /* create a upload list file src_list.txt */
        int file_attr = file_exists(handle);
   }

    /* create a upload list file src_list.txt */
    cli_api->src_fd = fopen(cli_api->src_list, "r");

    if (!cli_api->src_fd) {
        printf("[%s][%d]Invalid file path: %s\n",
                __FUNCTION__, __LINE__, cli_api->src_list);
        exit(0);
    } else {
        /* count total src_list files */
        char line[MAX_UPLOAD_FILES][256];
        char *temp;
        int i = 0x00;

        memset(line, 0x00, sizeof(line));

        /* read a line from a file */
        while (fgets(line[i], sizeof(line), cli_api->src_fd) != NULL) {
            if (i <= MAX_UPLOAD_FILES) {
                i++;
            } else {
                i = (i - 1);
                printf("[%s][%d] Upload files limit set to %d \n",
                       __FUNCTION__, __LINE__, (MAX_UPLOAD_FILES));
                break;
            }
        }

        total_src_files = i;
    }

    cli_api->total_files = total_src_files;
    cli_api->xfer_count = 0;

    cli_api->rcvd_cmd_resp = "verify-upload-files-resp";
    queue_next_cmd_req(cli_api);
}

static void download_file_complete(int status, FILE *fd, void *handle)
{
    cli_api_t *cli_api = handle;
    cli_api->rcvd_cmd_resp = "download-file-resp";

    printf("\n");
    fclose(fd);
    if (status) {
        // TODO send to stderr
        switch(status) {
            case STORJ_FILE_DECRYPTION_ERROR:
                printf("Unable to properly decrypt file, please check " \
                       "that the correct encryption key was " \
                       "imported correctly.\n\n");
                break;
            default:
                printf("[%s][%d]Download failure: %s\n",
                       __FUNCTION__, __LINE__, storj_strerror(status));
        }
    } else {
        printf("Download Success!\n");
    }

    queue_next_cmd_req(cli_api);
}

static void download_signal_handler(uv_signal_t *req, int signum)
{
    storj_download_state_t *state = req->data;
    storj_bridge_resolve_file_cancel(state);
    if (uv_signal_stop(req)) {
        printf("Unable to stop signal\n");
    }
    uv_close((uv_handle_t *)req, close_signal);
}

static int download_file(storj_env_t *env, char *bucket_id,
                         char *file_id, char *path, void *handle)
{
    FILE *fd = NULL;

    if (path) {
        char user_input[BUFSIZ];
        memset(user_input, '\0', BUFSIZ);

        if (access(path, F_OK) != -1 ) {
            printf("Warning: File already exists at path [%s].\n", path);
            while (strcmp(user_input, "y") != 0 && strcmp(user_input, "n") != 0) {
                memset(user_input, '\0', BUFSIZ);
                printf("Would you like to overwrite [%s]: [y/n] ", path);
                get_input(user_input);
            }

            if (strcmp(user_input, "n") == 0) {
                printf("\nCanceled overwriting of [%s].\n", path);
                cli_api_t *cli_api = handle;
                cli_api->rcvd_cmd_resp = "download-file-resp";
                queue_next_cmd_req(cli_api);
                return 1;
            }

            unlink(path);
        }

        fd = fopen(path, "w+");
    } else {
        fd = stdout;
    }

    if (fd == NULL) {
        // TODO send to stderr
        printf("Unable to open %s: %s\n", path, strerror(errno));
        return 1;
    }

    uv_signal_t *sig = malloc(sizeof(uv_signal_t));
    uv_signal_init(env->loop, sig);
    uv_signal_start(sig, download_signal_handler, SIGINT);

    storj_progress_cb progress_cb = (storj_progress_cb)noop;
    if (path && env->log_options->level == 0) {
        progress_cb = file_progress;
    }

    storj_download_state_t *state = storj_bridge_resolve_file(env, bucket_id,
                                                              file_id, fd, handle,
                                                              progress_cb,
                                                              download_file_complete);
    if (!state) {
        return 1;
    }
    sig->data = state;

    return state->error_status;
}

static void list_mirrors_callback(uv_work_t *work_req, int status)
{
    assert(status == 0);
    json_request_t *req = work_req->data;

    cli_api_t *cli_api = req->handle;
    cli_api->last_cmd_req = cli_api->curr_cmd_req;
    cli_api->rcvd_cmd_resp = "list-mirrors-resp";

    if (req->status_code != 200) {
        printf("Request failed with status code: %i\n",
               req->status_code);
        goto cleanup;
    }

    if (req->response == NULL) {
        free(req);
        free(work_req);
        printf("Failed to list mirrors.\n");
        goto cleanup;
    }

    int num_mirrors = json_object_array_length(req->response);

    struct json_object *shard;
    struct json_object *established;
    struct json_object *available;
    struct json_object *item;
    struct json_object *hash;
    struct json_object *contract;
    struct json_object *address;
    struct json_object *port;
    struct json_object *node_id;

    for (int i = 0; i < num_mirrors; i++) {
        shard = json_object_array_get_idx(req->response, i);
        json_object_object_get_ex(shard, "established",
                                 &established);
        int num_established =
            json_object_array_length(established);
        for (int j = 0; j < num_established; j++) {
            item = json_object_array_get_idx(established, j);
            if (j == 0) {
                json_object_object_get_ex(item, "shardHash",
                                          &hash);
                printf("Shard %i: %s\n", i, json_object_get_string(hash));
            }
            json_object_object_get_ex(item, "contract", &contract);
            json_object_object_get_ex(contract, "farmer_id", &node_id);

            const char *node_id_str = json_object_get_string(node_id);
            printf("\tnodeID: %s\n", node_id_str);
        }
        printf("\n\n");
    }

    json_object_put(req->response);

    queue_next_cmd_req(cli_api);
cleanup:
    free(req->path);
    free(req);
    free(work_req);
}

static void delete_file_callback(uv_work_t *work_req, int status)
{
    assert(status == 0);
    json_request_t *req = work_req->data;

    cli_api_t *cli_api = req->handle;
    cli_api->last_cmd_req = cli_api->curr_cmd_req;
    cli_api->rcvd_cmd_resp = "remove-file-resp";

    if (req->status_code == 200 || req->status_code == 204) {
        printf("File was successfully removed from bucket.\n");
    } else if (req->status_code == 401) {
        printf("Invalid user credentials.\n");
        goto cleanup;
    } else if (req->status_code == 403) {
        printf("Forbidden, user not active.\n");
        goto cleanup;
    } else {
        printf("Failed to remove file from bucket. (%i)\n", req->status_code);
        goto cleanup;
    }

    json_object_put(req->response);

    queue_next_cmd_req(cli_api);
cleanup:
    free(req->path);
    free(req);
    free(work_req);
}

static void delete_bucket_callback(uv_work_t *work_req, int status)
{
    assert(status == 0);
    json_request_t *req = work_req->data;

    cli_api_t *cli_api = req->handle;
    cli_api->last_cmd_req = cli_api->curr_cmd_req;
    cli_api->rcvd_cmd_resp = "remove-bucket-resp";

    if (req->status_code == 200 || req->status_code == 204) {
        printf("Bucket was successfully removed.\n");
    } else if (req->status_code == 401) {
        printf("Invalid user credentials.\n");
        goto cleanup;
    } else if (req->status_code == 403) {
        printf("Forbidden, user not active.\n");
        goto cleanup;
    } else {
        printf("Failed to destroy bucket. (%i)\n", req->status_code);
        goto cleanup;
    }

    json_object_put(req->response);

    queue_next_cmd_req(cli_api);
cleanup:
    free(req->path);
    free(req);
    free(work_req);
}

void get_buckets_callback(uv_work_t *work_req, int status)
{
    assert(status == 0);
    get_buckets_request_t *req = work_req->data;

    if (req->status_code == 401) {
       printf("Invalid user credentials.\n");
    } else if (req->status_code == 403) {
       printf("Forbidden, user not active.\n");
    } else if (req->status_code != 200 && req->status_code != 304) {
        printf("Request failed with status code: %i\n", req->status_code);
    } else if (req->total_buckets == 0) {
        printf("No buckets.\n");
    }

    for (int i = 0; i < req->total_buckets; i++) {
        storj_bucket_meta_t *bucket = &req->buckets[i];
        printf("ID: %s \tDecrypted: %s \tCreated: %s \tName: %s\n",
               bucket->id, bucket->decrypted ? "true" : "false",
               bucket->created, bucket->name);
    }

    storj_free_get_buckets_request(req);
    free(work_req);
}

void get_bucket_id_callback(uv_work_t *work_req, int status)
{
    int ret_status = 0x00;
    assert(status == 0);
    get_bucket_id_request_t *req = work_req->data;
    cli_api_t *cli_api = req->handle;

    cli_api->last_cmd_req = cli_api->curr_cmd_req;
    cli_api->rcvd_cmd_resp = "get-bucket-id-resp";

    if (req->status_code == 401) {
        printf("Invalid user credentials.\n");
        goto cleanup;
    } else if (req->status_code == 403) {
        printf("Forbidden, user not active.\n");
        goto cleanup;
    } else if (req->status_code != 200 && req->status_code != 304) {
        printf("Request failed with status code: %i\n", req->status_code);
        goto cleanup;
    }

    /* store the bucket id */
    memset(cli_api->bucket_id, 0x00, sizeof(cli_api->bucket_id));
    strcpy(cli_api->bucket_id, (char *)req->bucket_id);
    printf("ID: %s \tName: %s\n", req->bucket_id, req->bucket_name);

    queue_next_cmd_req(cli_api);

cleanup:
    free(req);
    free(work_req);
}

void get_file_id_callback(uv_work_t *work_req, int status)
{
    int ret_status = 0x00;
    assert(status == 0);
    get_file_id_request_t *req = work_req->data;
    cli_api_t *cli_api = req->handle;

    cli_api->last_cmd_req = cli_api->curr_cmd_req;
    cli_api->rcvd_cmd_resp = "get-file-id-resp";

    if (req->status_code == 401) {
        printf("Invalid user credentials.\n");
        goto cleanup;
    } else if (req->status_code == 403) {
        printf("Forbidden, user not active.\n");
        goto cleanup;
    } else if (req->status_code != 200 && req->status_code != 304) {
        printf("Request failed with status code: %i\n", req->status_code);
        goto cleanup;
    }

    /* store the bucket id */
    memset(cli_api->file_id, 0x00, sizeof(cli_api->file_id));
    strcpy(cli_api->file_id, (char *)req->file_id);
    printf("ID: %s \tName: %s\n", req->file_id, req->file_name);

    queue_next_cmd_req(cli_api);

    cleanup:
    free(req);
    free(work_req);
}

void list_files_callback(uv_work_t *work_req, int status)
{
    int ret_status = 0;
    assert(status == 0);
    list_files_request_t *req = work_req->data;

    cli_api_t *cli_api = req->handle;
    cli_api->last_cmd_req = cli_api->curr_cmd_req;
    cli_api->rcvd_cmd_resp = "list-files-resp";

    if (req->status_code == 404) {
        printf("Bucket id [%s] does not exist\n", req->bucket_id);
        goto cleanup;
    } else if (req->status_code == 400) {
        printf("Bucket id [%s] is invalid\n", req->bucket_id);
        goto cleanup;
    } else if (req->status_code == 401) {
        printf("Invalid user credentials.\n");
        goto cleanup;
    } else if (req->status_code == 403) {
        printf("Forbidden, user not active.\n");
        goto cleanup;
    } else if (req->status_code != 200) {
        printf("Request failed with status code: %i\n", req->status_code);
    }

    if (req->total_files == 0) {
        printf("No files for bucket.\n");
        goto cleanup;
    }

    cli_api->files = malloc(sizeof(storj_file_meta_t) * req->total_files);

    for (int i = 0; i < req->total_files; i++) {

        storj_file_meta_t *file = &req->files[i];

        cli_api->files[i].id = strdup(file->id);
        cli_api->files[i].size = file->size;
        cli_api->files[i].filename = strdup(file->id);
        cli_api->files[i].decrypted = file->decrypted;
        cli_api->files[i].mimetype = strdup(file->mimetype);
        cli_api->files[i].created = strdup(file->created);

        printf("ID: %s \tSize: %" PRIu64 " bytes \tDecrypted: %s \tType: %s \tCreated: %s \tName: %s\n",
               file->id,
               file->size,
               file->decrypted ? "true" : "false",
               file->mimetype,
               file->created,
               file->filename);
    }


    cli_api->total_files = req->total_files;
    cli_api->xfer_count = 0;
    queue_next_cmd_req(cli_api);

  cleanup:

    storj_free_list_files_request(req);
    free(work_req);
}

void queue_next_cmd_req(cli_api_t *cli_api)
{
    void *handle = cli_api->handle;

    #ifdef debug_enable
    printf("[%s][%d]start !!!! expt resp = %s; rcvd resp = %s \n",
           __FUNCTION__, __LINE__,
            cli_api->excp_cmd_resp, cli_api->rcvd_cmd_resp );
    printf("[%s][%d]last cmd = %s; cur cmd = %s; next cmd = %s\n",
           __FUNCTION__, __LINE__, cli_api->last_cmd_req,
           cli_api->curr_cmd_req, cli_api->next_cmd_req);
    #endif

    if (cli_api->excp_cmd_resp != NULL) {
        if (strcmp(cli_api->excp_cmd_resp, cli_api->rcvd_cmd_resp) == 0x00) {
            if ((cli_api->next_cmd_req != NULL) &&
                (strcmp(cli_api->next_cmd_req, "get-file-id-req") == 0x00)) {
                cli_api->curr_cmd_req  = cli_api->next_cmd_req;
                cli_api->next_cmd_req  = cli_api->final_cmd_req;
                cli_api->final_cmd_req = NULL;
                cli_api->excp_cmd_resp = "get-file-id-resp";

                storj_bridge_get_file_id(cli_api->env, cli_api->bucket_id,
                                        cli_api->file_name, cli_api, get_file_id_callback);
            } else if ((cli_api->next_cmd_req != NULL) &&
                (strcmp(cli_api->next_cmd_req, "list-files-req") == 0x00)) {
                cli_api->curr_cmd_req  = cli_api->next_cmd_req;
                cli_api->next_cmd_req  = cli_api->final_cmd_req;
                cli_api->final_cmd_req = NULL;
                cli_api->excp_cmd_resp = "list-files-resp";

                storj_bridge_list_files(cli_api->env, cli_api->bucket_id,
                                        cli_api, list_files_callback);
            } else if ((cli_api->next_cmd_req != NULL) &&
                     (strcmp(cli_api->next_cmd_req, "remove-bucket-req") == 0x00)) {
                cli_api->curr_cmd_req  = cli_api->next_cmd_req;
                cli_api->next_cmd_req  = cli_api->final_cmd_req;
                cli_api->final_cmd_req = NULL;
                cli_api->excp_cmd_resp = "remove-bucket-resp";

                storj_bridge_delete_bucket(cli_api->env, cli_api->bucket_id,
                                           cli_api, delete_bucket_callback);
            } else if ((cli_api->next_cmd_req != NULL) &&
                     (strcmp(cli_api->next_cmd_req, "remove-file-req") == 0x00)) {
                printf("[%s][%d]file-name = %s; file-id = %s; bucket-name = %s \n",
                        __FUNCTION__, __LINE__, cli_api->file_name, cli_api->file_id,
                        cli_api->bucket_name);

                cli_api->curr_cmd_req  = cli_api->next_cmd_req;
                cli_api->next_cmd_req  = cli_api->final_cmd_req;
                cli_api->final_cmd_req = NULL;
                cli_api->excp_cmd_resp = "remove-file-resp";

                storj_bridge_delete_file(cli_api->env, cli_api->bucket_id, cli_api->file_id,
                                            cli_api, delete_file_callback);
            } else if ((cli_api->next_cmd_req != NULL) &&
                     (strcmp(cli_api->next_cmd_req, "list-mirrors-req") == 0x00)) {
                printf("[%s][%d]file-name = %s; file-id = %s; bucket-name = %s \n",
                        __FUNCTION__, __LINE__, cli_api->file_name, cli_api->file_id,
                        cli_api->bucket_name);

                cli_api->curr_cmd_req  = cli_api->next_cmd_req;
                cli_api->next_cmd_req  = cli_api->final_cmd_req;
                cli_api->final_cmd_req = NULL;
                cli_api->excp_cmd_resp = "list-mirrors-resp";

                storj_bridge_list_mirrors(cli_api->env, cli_api->bucket_id, cli_api->file_id,
                                            cli_api, list_mirrors_callback);
            } else if ((cli_api->next_cmd_req != NULL) &&
                     (strcmp(cli_api->next_cmd_req, "upload-file-req") == 0x00)) {
                cli_api->curr_cmd_req  = cli_api->next_cmd_req;
                cli_api->next_cmd_req  = cli_api->final_cmd_req;
                cli_api->final_cmd_req = NULL;
                cli_api->excp_cmd_resp = "upload-file-resp";

                upload_file(cli_api->env, cli_api->bucket_id, cli_api->file_name, cli_api);
            } else if ((cli_api->next_cmd_req != NULL) &&
                     (strcmp(cli_api->next_cmd_req, "verify-upload-files-req") == 0x00)) {
                cli_api->curr_cmd_req  = cli_api->next_cmd_req;
                cli_api->next_cmd_req  = cli_api->final_cmd_req;
                cli_api->final_cmd_req = NULL;
                cli_api->excp_cmd_resp = "verify-upload-files-resp";

                verify_upload_files(cli_api);
            } else if ((cli_api->next_cmd_req != NULL) &&
                     (strcmp(cli_api->next_cmd_req, "upload-files-req") == 0x00)) {
                cli_api->curr_cmd_req  = cli_api->next_cmd_req;
                cli_api->excp_cmd_resp = "upload-files-resp";

                FILE *file = fopen(cli_api->src_list, "r");

                char line[256][256];
                char *temp;
                int i = 0x00;
                memset(line, 0x00, sizeof(line));

                if (file != NULL) {
                    while ((fgets(line[i],sizeof(line), file)!= NULL)) {/* read a line from a file */
                        temp = strrchr(line[i], '\n');
                        if (temp) *temp = '\0';
                        cli_api->src_file = line[i];
                        i++;
                        if (i >= cli_api->xfer_count) {
                            break;
                        }
                    }
                }
                fclose(file);

                if (cli_api->xfer_count < cli_api->total_files) {
                    /* is it the last file ? */
                    if (cli_api->xfer_count == cli_api->total_files - 1) {
                        cli_api->next_cmd_req  = cli_api->final_cmd_req;
                        cli_api->final_cmd_req = NULL;
                    }

                    upload_files(cli_api->env, cli_api->bucket_id, cli_api->src_file, cli_api);
                    cli_api->xfer_count++;
                } else {
                    printf("[%s][%d] Invalid xfer counts\n", __FUNCTION__, __LINE__);
                    exit(0);
                }
            } else if ((cli_api->next_cmd_req != NULL) &&
                     (strcmp(cli_api->next_cmd_req, "download-file-req") == 0x00)) {
                cli_api->curr_cmd_req  = cli_api->next_cmd_req;
                cli_api->next_cmd_req  = cli_api->final_cmd_req;
                cli_api->final_cmd_req = NULL;
                cli_api->excp_cmd_resp = "download-file-resp";

                download_file(cli_api->env, cli_api->bucket_id, cli_api->file_id,
                                cli_api->dst_file, cli_api);
            } else if ((cli_api->next_cmd_req != NULL) &&
                     (strcmp(cli_api->next_cmd_req, "download-files-req") == 0x00)) {
                cli_api->curr_cmd_req  = cli_api->next_cmd_req;
                cli_api->excp_cmd_resp = "download-file-resp";

                FILE *file = stdout;

                if (cli_api->xfer_count < cli_api->total_files) {

                    storj_file_meta_t *file = &cli_api->files[cli_api->xfer_count];

                    /* is it the last file ? */
                    if (cli_api->xfer_count == cli_api->total_files - 1) {
                        cli_api->next_cmd_req  = cli_api->final_cmd_req;
                        cli_api->final_cmd_req = NULL;
                    }

                    memset(cli_api->file_id, 0x00, sizeof(file->id));
                    strcpy(cli_api->file_id, file->id);

                    char temp_path[1024];

                    strcpy(temp_path, cli_api->file_path);
                    if (cli_api->file_path[(strlen(cli_api->file_path)-1)] != '/') {
                        strcat(temp_path, "/");
                    }
                    strcat(temp_path, file->filename);

                    cli_api->xfer_count++;
                    download_file(cli_api->env, cli_api->bucket_id, cli_api->file_id, temp_path, cli_api);

                    fprintf(stdout,"*****[%d:%d] downloading file to: %s *****\n", cli_api->xfer_count, cli_api->total_files, temp_path);
                } else {
                    printf("[%s][%d] Invalid xfer counts\n", __FUNCTION__, __LINE__);
                    exit(0);
                }

            } else {
                #ifdef debug_enable
                printf("[%s][%d] **** ALL CLEAN & DONE  *****\n", __FUNCTION__, __LINE__);
                #endif

                exit(0);
            }
        } else {
            printf("[%s][%d]Oops !!!! expt resp = %s; rcvd resp = %s \n",
                   __FUNCTION__, __LINE__,
                    cli_api->excp_cmd_resp, cli_api->rcvd_cmd_resp );
            printf("[%s][%d]last cmd = %s; cur cmd = %s; next cmd = %s\n",
                   __FUNCTION__, __LINE__, cli_api->last_cmd_req,
                   cli_api->curr_cmd_req, cli_api->next_cmd_req);
        }
    } else {
        /* calling straight storj calls without going thru the state machine */
        exit(0);
    }
}

int cli_list_buckets(cli_api_t *cli_api)
{
    cli_api->last_cmd_req  = NULL;
    cli_api->curr_cmd_req  = "get-bucket-id-req";
    cli_api->next_cmd_req  = NULL;
    cli_api->final_cmd_req = NULL;
    cli_api->excp_cmd_resp = "get-bucket-id-resp";

    /* when callback returns, we store the bucket id of bucket name else null */
    return storj_bridge_get_buckets(cli_api->env, cli_api, get_buckets_callback);
}

int cli_get_bucket_id(cli_api_t *cli_api)
{
    int ret = -1;
    cli_api->last_cmd_req  = NULL;
    cli_api->curr_cmd_req  = "get-bucket-id-req";
    cli_api->next_cmd_req  = NULL;
    cli_api->final_cmd_req = NULL;
    cli_api->excp_cmd_resp = "get-bucket-id-resp";

    /* when callback returns, we store the bucket id of bucket name else null */
    //return storj_bridge_get_buckets(cli_api->env, cli_api, get_bucket_id_callback);
    return storj_bridge_get_bucket_id(cli_api->env, cli_api->bucket_name, cli_api, get_bucket_id_callback);
}

int cli_get_file_id(cli_api_t *cli_api)
{
    int ret = 0x00;
    ret = cli_get_bucket_id(cli_api);
    cli_api->next_cmd_req  = "get-file-id-req";
    cli_api->final_cmd_req = NULL;

    return ret;
}

int cli_list_files(cli_api_t *cli_api)
{
    int ret = 0x00;
    ret = cli_get_bucket_id(cli_api);
    cli_api->next_cmd_req  = "list-files-req";
    cli_api->final_cmd_req = NULL;

    return ret;
}

int cli_remove_bucket(cli_api_t *cli_api)
{
    int ret = 0x00;
    ret = cli_get_bucket_id(cli_api);
    cli_api->next_cmd_req  = "remove-bucket-req";
    cli_api->final_cmd_req = NULL;

    return ret;
}

int cli_remove_file(cli_api_t *cli_api)
{
    int ret = 0x00;
    ret = cli_get_file_id(cli_api);
    cli_api->final_cmd_req  = "remove-file-req";

    return ret;
}

int cli_list_mirrors(cli_api_t *cli_api)
{
    int ret = 0x00;
    ret = cli_get_file_id(cli_api);
    cli_api->final_cmd_req  = "list-mirrors-req";

    return ret;
}

int cli_upload_file(cli_api_t *cli_api)
{
    int ret = 0x00;
    ret = cli_get_bucket_id(cli_api);
    cli_api->next_cmd_req  = "upload-file-req";
    cli_api->final_cmd_req = NULL;

    return ret;
}

int cli_upload_files(cli_api_t *cli_api)
{
    int ret = 0x00;
    ret = cli_get_bucket_id(cli_api);
    cli_api->next_cmd_req  = "verify-upload-files-req";
    cli_api->final_cmd_req = "upload-files-req";

    return ret;
}

int cli_download_file(cli_api_t *cli_api)
{
    int ret = 0x00;
    ret = cli_get_file_id(cli_api);
    cli_api->final_cmd_req  = "download-file-req";

    return ret;
}

int cli_download_files(cli_api_t *cli_api)
{
    int ret = 0x00;
    ret = cli_list_files(cli_api);
    cli_api->final_cmd_req  = "download-files-req";

    return ret;
}
