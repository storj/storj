package storj

/*

#include "storj.h"

//------------------------------------------------------------------------------
// The gateway function
//------------------------------------------------------------------------------
void storj_uv_run_cgo(storj_env_t *env)
{
	printf("entering into storj_uv_run_cgo()\n");
	  // run all queued events
	  if (uv_run(env->loop, UV_RUN_DEFAULT)) {
		printf("inside uv_run()\n");
        uv_loop_close(env->loop);

        // cleanup
        storj_destroy_env(env);
	}
	printf("done with storj_uv_run_cgo()\n");
}

//------------------------------------------------------------------------------
// Returns the pointer to the array at the index
//------------------------------------------------------------------------------
storj_bucket_meta_t *bucket_index(storj_bucket_meta_t *array, int index) {
  return &array[index];
}

//------------------------------------------------------------------------------
// Returns the pointer to the array at the index
//------------------------------------------------------------------------------
storj_file_meta_t *file_index(storj_file_meta_t *array, int index) {
  return &array[index];
}

int upload_file(storj_env_t *env, char *bucket_id, const char *file_path, char *file_name, void *handle)
{
		FILE *fd = fopen(file_path, "r");
		printf ("[%s][%s] file_path = %s\n", __FUNCTION__, __FILE__, file_path);

    if (!fd) {
        printf("Invalid file path: %s\n", file_path);
        exit(0);
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

    storj_upload_state_t *state = storj_bridge_store_file(env,
                                                          &upload_opts,
                                                          handle,
                                                          NULL, //progress_cb,
                                                          NULL);

    if (!state) {
        return 1;
    }

    return state->error_status;
}

void file_open_test(void)
{
	FILE *ptr_file;
	char buf[1000];

	ptr_file =fopen("/Users/kishore/Downloads/upload_testfile.txt","r");

	while (fgets(buf,1000, ptr_file)!=NULL)
					printf("%s",buf);

	fclose(ptr_file);
}

*/
import "C"
