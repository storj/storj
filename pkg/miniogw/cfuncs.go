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
*/
import "C"
