// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdlib.h>

#include "require.h"
#include "uplink.h"
#include "helpers2.h"

void HandleProject(Project project);

int main(int argc, char *argv[]) {
    WithTestProject(&HandleProject);
}

void HandleProject(Project project) {
    char *_err = "";
    char **err = &_err;

    char *bucket_name = "TestBucket";

    BucketConfig config = TestBucketConfig();
    BucketInfo info = CreateBucket(project, bucket_name, &config, err);
    require_noerror(*err);
    FreeBucketInfo(&info);

    Bucket bucket = OpenBucket(project, bucket_name, NULL, err);
    require_noerror(*err);
    {
        char *object_paths[] = {"TestObject1","TestObject2","TestObject3","TestObject4"};
        int num_of_objects = 4;

        char *data = "testing data 123";
        //for(int i = 0; i < num_of_objects; i++) {
        //    
        //}
    }
    CloseBucket(bucket, err);
    require_noerror(*err);
}
