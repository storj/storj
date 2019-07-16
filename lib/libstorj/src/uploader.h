/**
 * @file uploader.h
 * @brief Storj upload methods and definitions.
 *
 * Structures and functions useful for uploading files.
 */
#ifndef STORJ_UPLOADER_H
#define STORJ_UPLOADER_H
#include "storj.h"

#define STORJ_NULL -1
#define STORJ_MAX_REPORT_TRIES 2
#define STORJ_MAX_PUSH_FRAME_COUNT 6

static uv_work_t *uv_work_new();

static int check_in_progress(storj_upload_state_t *state, int status);
// TODO: is this still used?
char *create_tmp_name(storj_upload_state_t *state, char *extension);

static void cleanup_state(storj_upload_state_t *state);

#endif /* STORJ_UPLOADER_H */
